package dsl

import (
	"github.com/ccbhj/gendsl"
	"github.com/ccbhj/grete"
	"github.com/ccbhj/grete/internal/rete"
	"github.com/ccbhj/grete/internal/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("DefineProduction", func() {
	Describe("When", func() {
		It("can define a string alias", func() {
			script := `
(define-prdt test
	(when 
			(for-string foo (eq $$ "BAR") ))
	#:desc "some description"
)
			`

			result, err := gendsl.EvalExpr(script, globalEnv)
			Expect(err).Should(BeNil())

			prd, ok := result.Unwrap().(grete.Production)
			Expect(ok).Should(BeTrue())
			Expect(prd).Should(BeEquivalentTo(grete.Production{
				Desc: "some description",
				Production: rete.Production{
					ID: "test",
					When: []rete.AliasDeclaration{
						{
							Alias: "foo",
							Type: types.TypeInfo{
								T: types.GValueTypeString,
							},
							Guards: []rete.Guard{
								{
									AliasAttr: types.FieldSelf,
									Value:     types.GVString("BAR"),
								},
							},
						},
					},
				},
			}))
		})
	})
})
