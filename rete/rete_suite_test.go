package rete_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestRete(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Rete Suite")
}
