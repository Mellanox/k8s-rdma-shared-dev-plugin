package resources

import (
	"os"
	"os/signal"

	"github.com/Mellanox/k8s-rdma-shared-dev-plugin/pkg/types"

	"github.com/fsnotify/fsnotify"
)

type signalNotifier struct {
	signals []os.Signal
}

func (n *signalNotifier) Notify() chan os.Signal {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, n.signals...)

	return sigChan
}

func newFSWatcher(files ...string) (*fsnotify.Watcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	for _, f := range files {
		err = watcher.Add(f)
		if err != nil {
			watcher.Close()
			return nil, err
		}
	}

	return watcher, nil
}

// NewSignalNotifier return signals notifier
func NewSignalNotifier(sigs ...os.Signal) types.SignalNotifier {
	return &signalNotifier{signals: sigs}
}
