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
