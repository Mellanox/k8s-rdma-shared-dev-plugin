package resources

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type mockedSignalNotifier struct {
	signals     []os.Signal
	triggerChan chan os.Signal
}

func (n *mockedSignalNotifier) Notify() chan os.Signal {
	sigChan := make(chan os.Signal, 1)
	n.triggerChan = sigChan
	return sigChan
}

type mockedSignal struct {
	message string
}

func (s *mockedSignal) String() string {
	return s.message
}
func (s *mockedSignal) Signal() {}

func (n *mockedSignalNotifier) triggerSignal(signal os.Signal) {
	for _, s := range n.signals {
		if s == signal {
			n.triggerChan <- s
		}
	}
}

var _ = Describe("Watcher", func() {
	Context("NewOSWatcher", func() {
		It("Watcher for signals", func() {
			sn := mockedSignalNotifier{}
			sign := &mockedSignal{message: "mocked signal"}
			sn.signals = append(sn.signals, sign)

			finishChan := make(chan bool)

			signsChannel := sn.Notify()
			go func() {
				s := <-signsChannel
				Expect(s.String()).To(Equal(sign.message))
				finishChan <- true
			}()
			sn.triggerSignal(sign)

			result := <-finishChan
			Expect(result).To(BeTrue())
		})
	})
})
