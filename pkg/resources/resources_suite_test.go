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
	"log"
	"os"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var testCallsAssertionReporter = &testingT{}

// Need to assert mocked functions are called
type testingT struct {
}

func (t *testingT) Logf(format string, args ...interface{}) {
	log.Printf(format, args...)
}
func (t *testingT) Errorf(format string, args ...interface{}) {
	log.Fatalf(format, args...)
}
func (t *testingT) FailNow() {
	os.Exit(1)
}

func TestConfig(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Resources test suite")
}
