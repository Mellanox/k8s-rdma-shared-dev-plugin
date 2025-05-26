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
