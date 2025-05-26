// Copyright 2025 NVIDIA CORPORATION & AFFILIATES
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

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
