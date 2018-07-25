package base

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/docker/docker/client"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/replicatedhq/ship/pkg/cli"
	"gopkg.in/yaml.v2"
	"github.com/replicatedhq/ship/integration"
)

type TestMetadata struct {
	CustomerID        string `yaml:"customer_id"`
	InstallationID    string `yaml:"installation_id"`
	ReleaseVersion    string `yaml:"release_version"`
	StudioChannelName string `yaml:"studio_channel_name"`
	Flavor            string `yaml:"flavor"`

	//debugging
	SkipCleanup bool `yaml:"skip_cleanup"`
}

func TestCore(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "integration")
}

var _ = Describe("basic", func() {
	dockerClient, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}
	dockerClient.NegotiateAPIVersion(context.Background())

	integrationDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	files, err := ioutil.ReadDir(integrationDir)
	if err != nil {
		panic(err)
	}

	for _, file := range files {
		if file.IsDir() {
			Context(fmt.Sprintf("When the spec in %q is run", file.Name()), func() {
				testPath := path.Join(integrationDir, file.Name())
				testInputPath := path.Join(testPath, "input")
				var testOutputPath string
				var testMetadata TestMetadata

				BeforeEach(func() {
					// create a temporary directory within this directory to compare files with
					testOutputPath, err = ioutil.TempDir(testPath, "test")
					Expect(err).NotTo(HaveOccurred())
					os.Chdir(testOutputPath)

					// read the test metadata
					testMetadata = readMetadata(testPath)
				}, 20)

				AfterEach(func() {
					if !testMetadata.SkipCleanup {
						// remove the temporary directory
						err := os.RemoveAll(testOutputPath)
						Expect(err).NotTo(HaveOccurred())
					}
					os.Chdir(integrationDir)
				}, 20)

				It("Should output files matching those expected when running in local mode", func() {
					cmd := cli.RootCmd()
					buf := new(bytes.Buffer)
					cmd.SetOutput(buf)
					cmd.SetArgs([]string{
						"--headless",
						fmt.Sprintf("--studio-file=%s", path.Join(testInputPath, ".ship/release.yml")),
						fmt.Sprintf("--state-file=%s", path.Join(testInputPath, ".ship/state.json")),
						fmt.Sprintf("--studio-channel-name=%s", testMetadata.StudioChannelName),
						fmt.Sprintf("--release-semver=%s", testMetadata.ReleaseVersion),
						"--log-level=off",
						"--terraform-yes",
					})
					err := cmd.Execute()
					Expect(err).NotTo(HaveOccurred())

					//compare the files in the temporary directory with those in the "expected" directory
					result, err := integration.CompareDir(path.Join(testPath, "expected"), testOutputPath)
					Expect(err).NotTo(HaveOccurred())
					Expect(result).To(BeTrue())
				}, 60)

				It("Should output files matching those expected when communicating with the graphql api", func() {
					cmd := cli.RootCmd()
					buf := new(bytes.Buffer)
					cmd.SetOutput(buf)
					cmd.SetArgs(append([]string{
						"--headless",
						fmt.Sprintf("--state-file=%s", path.Join(testInputPath, ".ship/state.json")),
						"--customer-endpoint=https://pg.staging.replicated.com/graphql",
						"--log-level=off",
						fmt.Sprintf("--customer-id=%s", testMetadata.CustomerID),
						fmt.Sprintf("--installation-id=%s", testMetadata.InstallationID),
						fmt.Sprintf("--release-semver=%s", testMetadata.ReleaseVersion),
						"--terraform-yes",
					}))
					err := cmd.Execute()
					Expect(err).NotTo(HaveOccurred())

					//compare the files in the temporary directory with those in the "expected" directory
					result, err := integration.CompareDir(path.Join(testPath, "expected"), testOutputPath)
					Expect(err).NotTo(HaveOccurred())
					Expect(result).To(BeTrue())
				}, 60)
			})
		}
	}
})

func readMetadata(testPath string) TestMetadata {
	var testMetadata TestMetadata
	metadataBytes, err := ioutil.ReadFile(path.Join(testPath, "metadata.yaml"))
	Expect(err).NotTo(HaveOccurred())
	err = yaml.Unmarshal(metadataBytes, &testMetadata)
	Expect(err).NotTo(HaveOccurred())

	return testMetadata
}