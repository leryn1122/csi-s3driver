package driver

import (
	"fmt"
	"log"
	"os"
	"path"
	"testing"

	"github.com/kubernetes-csi/csi-test/v5/pkg/sanity"
	"github.com/leryn1122/csi-s3/pkg/constant"
	. "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
)

func TestS3Driver(t *testing.T) {
	o.RegisterFailHandler(Fail)
	RunSpecs(t, "csi-s3driver")
}

var _ = Describe("csi-s3driver", func() {
	Context("s3fs", func() {
		socket := fmt.Sprintf("/tmp/%s.sock", constant.DriverName)
		csiEndpoint := "unix://" + socket
		if err := os.Remove(socket); err != nil && !os.IsNotExist(err) {
			o.Expect(err).NotTo(o.HaveOccurred())
		}
		driver, err := New("test-node", csiEndpoint)
		if err != nil {
			log.Fatal(err)
		}
		go driver.Run()

		Describe("CSI sanity", func() {
			sanityConfig := &sanity.TestConfig{
				TargetPath:  path.Join(os.TempDir(), "s3fs-target"),
				StagingPath: path.Join(os.TempDir(), "s3fs-staging"),
				Address:     csiEndpoint,
				SecretsFile: "../../test/secret.yaml",
				TestVolumeParameters: map[string]string{
					"mounter": "s3fs",
					"bucket":  "test",
				},
			}
			sanity.GinkgoTest(sanityConfig)
		})
	})
})
