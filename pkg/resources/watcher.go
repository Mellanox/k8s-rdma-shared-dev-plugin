package resources

import (
	"os"
	"os/signal"

	"github.com/Mellanox/k8s-rdma-shared-dev-plugin/pkg/types"
)

type signalNotifier struct {
	signals []os.Signal
}

func (n *signalNotifier) Notify() chan os.Signal {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, n.signals...)

	return sigChan
}

// NewSignalNotifier return signals notifier
func NewSignalNotifier(sigs ...os.Signal) types.SignalNotifier {
	return &signalNotifier{signals: sigs}
}
