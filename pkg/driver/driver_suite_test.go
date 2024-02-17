package driver

import (
	"fmt"
	"github.com/inhies/go-bytesize"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"log"
	"os"
	"testing"

	"github.com/kubernetes-csi/csi-test/v5/pkg/sanity"
	. "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
)

func TestS3Driver(t *testing.T) {
	o.RegisterFailHandler(Fail)
	RunSpecs(t, "csi-s3driver")
}

var _ = Describe("csi-s3driver", func() {
	Context("s3fs", func() {
		socket := fmt.Sprintf("/tmp/%s.sock", DriverName)
		csiEndpoint := "unix://" + socket

		if err := os.Remove(socket); err != nil && !os.IsNotExist(err) {
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		driver, err := NewDriver("unittest-node", csiEndpoint)
		if err != nil {
			log.Fatal(err)
		}
		go driver.Run()

		Describe("CSI sanity", func() {
			sanityConfig := &sanity.TestConfig{
				Address: csiEndpoint,
				DialOptions: []grpc.DialOption{
					grpc.WithTransportCredentials(insecure.NewCredentials()),
				},
				ControllerDialOptions: []grpc.DialOption{
					grpc.WithTransportCredentials(insecure.NewCredentials()),
				},
				SecretsFile:    "../../test/secret.yaml",
				TestVolumeSize: int64(512 * bytesize.MB), // 512 MB
			}
			sanity.GinkgoTest(sanityConfig)
		})
	})
})
