package resources

import (
	"github.com/Mellanox/k8s-rdma-shared-dev-plugin/pkg/utils"
	"os"
	"syscall"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Server", func() {
	Context("newFSWatcher", func() {
		It("Watcher for existing Dir", func() {
			fs := utils.FakeFilesystem{}
			defer fs.Use()()
			fsw, err := newFSWatcher(fs.RootDir)
			Expect(err).ToNot(HaveOccurred())
			Expect(fsw).ToNot(BeNil())
		})
		It("Watcher for non-existing Dir", func() {
			fsw, err := newFSWatcher("fake")
			Expect(err).To(HaveOccurred())
			Expect(fsw).To(BeNil())
		})
	})
	Context("NewOSWatcher", func() {
		It("Watcher for signals", func() {
			sw := NewOSWatcher(syscall.SIGHUP)
			go func() {
				p, err := os.FindProcess(os.Getpid())
				Expect(err).ToNot(HaveOccurred())
				err = p.Signal(syscall.SIGHUP)
				Expect(err).ToNot(HaveOccurred())
			}()
			s := <-sw
			Expect(s).To(Equal(syscall.SIGHUP))
		})
	})
})
