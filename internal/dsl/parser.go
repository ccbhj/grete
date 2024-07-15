package dsl

import (
	"github.com/ccbhj/gendsl"
	"github.com/pkg/errors"

	"github.com/ccbhj/grete"
	"github.com/ccbhj/grete/internal/rete"
	"github.com/ccbhj/grete/internal/types"
)

var globalEnv = gendsl.NewEnv().WithProcedure("define-prdt", gendsl.Procedure{Eval: _production})

var definePrdtEnv = gendsl.NewEnv().WithProcedure("when", gendsl.Procedure{Eval: _when})

type productionOpt func(*grete.Production) error

// Example:
//
//	  (define-prdt ExampleRule
//		  #:desc "find the prime number between 1-100"
//		  (when
//		    (for-struct addr (eq @addressLine1 "nowhere"))
//		    (for-struct is-struct (eq @name "Salaboy") )
//		  )
//		  (then
//		   (logf "Hey I just find %s that lives in %s" $person $addr))
//			)
func _production(evalCtx *gendsl.EvalCtx, args []gendsl.Expr, options map[string]gendsl.Value) (gendsl.Value, error) {
	prd := grete.Production{}

	// handle options
	desc, ok := options["desc"]
	if ok {
		if desc.Type() != gendsl.ValueTypeString {
			return nil, errors.Errorf("option #desc must be an string, but got %s", desc.Type())
		}
		prd.Desc = desc.Unwrap().(string)
	}

	if args[0].Type() != gendsl.ExprTypeIdentifier {
		return nil, errors.Errorf("arg #1 must be an identifier, but got %s", args[0].Type())
	}
	name := args[0].Text()
	prd.ID = name

	for _, arg := range args[1:] {
		opt, err := arg.EvalWithEnv(definePrdtEnv)
		if err != nil {
			return nil, err
		}
		optFn, ok := opt.Unwrap().(productionOpt)
		if !ok {
			panic("expecting a productionOpt")
		}
		if err := optFn(&prd); err != nil {
			return nil, err
		}
	}

	return &gendsl.UserData{V: prd}, nil
}

type selectorGenerator struct {
	ID types.GVIdentity
	T  types.TypeInfo
}

var _ gendsl.Selector = (*selectorGenerator)(nil)

func (s *selectorGenerator) Select(idx string) (gendsl.Value, bool) {
	_, in := s.T.Fields[idx]
	if !in {
		return nil, false
	}

	return &gendsl.UserData{
		V: &rete.Selector{
			Alias:     s.ID,
			AliasAttr: types.GVString(idx),
		},
	}, true
}

var condEnv = gendsl.NewEnv().
	WithProcedure("eq", gendsl.Procedure{Eval: makeCond(rete.TestOpEqual, false)})

func makeAliasDeclaration(t types.GValueType) gendsl.ProcedureFn {
	//(for-int addr (eq @ > 10))
	fn := func(evalCtx *gendsl.EvalCtx, args []gendsl.Expr, options map[string]gendsl.Value) (gendsl.Value, error) {
		decl := rete.AliasDeclaration{
			Type: types.TypeInfo{
				T: t,
			},
		}

		if args[0].Type() != gendsl.ExprTypeIdentifier {
			return nil, errors.Errorf("arg #1 must be an identifier, but got %s", args[0].Type())
		}
		name := args[0].Text()
		decl.Alias = types.GVIdentity(name)

		for _, condExpr := range args[1:] {
			cond, err := condExpr.EvalWithEnv(condEnv)
			if err != nil {
				return nil, err
			}
			switch v := cond.Unwrap().(type) {
			case rete.Guard:
				decl.Guards = append(decl.Guards, v)
			case rete.JoinTest:
				decl.JoinTests = append(decl.JoinTests, v)
			}
		}
		return &gendsl.UserData{V: decl}, nil
	}

	return gendsl.CheckNArgs("+", fn)
}

func makeCond(op rete.TestOp, negative bool) gendsl.ProcedureFn {
	fn := func(evalCtx *gendsl.EvalCtx, args []gendsl.Expr, options map[string]gendsl.Value) (gendsl.Value, error) {
		var (
			attr    types.GVString
			isGuard = true
			val     types.GValue
			refer   *rete.Selector
		)

		for _, arg := range args {
			switch arg.Type() {
			case gendsl.ExprTypeLiteral:
				cnst, err := arg.Eval()
				if err != nil {
					return nil, err
				}
				val = dslValueToGValue(cnst)
			case gendsl.ExprTypeIdentifier:
				val, err := arg.Eval()
				if err != nil {
					return nil, err
				}
				switch val.Type() {
				case gendsl.ValueTypeString: // attr
					attr = types.GVString(val.Unwrap().(string))
				case gendsl.ValueTypeUserData: // identifier
					isGuard = false
					refer = val.Unwrap().(*rete.Selector)
				}
			}
		}

		if isGuard {
			return &gendsl.UserData{
				V: rete.Guard{
					AliasAttr: attr,
					Value:     val,
					Negative:  negative,
					TestOp:    op,
				},
			}, nil
		}

		return &gendsl.UserData{
			V: rete.JoinTest{
				XAttr:  attr,
				Y:      *refer,
				TestOp: op,
			},
		}, nil
	}

	return gendsl.CheckNArgs("2", fn)
}

var typeEnv = gendsl.NewEnv().
	WithProcedure("for-int", gendsl.Procedure{Eval: makeAliasDeclaration(types.GValueTypeInt)}).
	WithProcedure("for-uint", gendsl.Procedure{Eval: makeAliasDeclaration(types.GValueTypeUint)}).
	WithProcedure("for-string", gendsl.Procedure{Eval: makeAliasDeclaration(types.GValueTypeString)}).
	WithProcedure("for-bool", gendsl.Procedure{Eval: makeAliasDeclaration(types.GValueTypeBool)}).
	WithProcedure("for-float", gendsl.Procedure{Eval: makeAliasDeclaration(types.GValueTypeFloat)})

//	 Conditions that returns AliasDeclaration
//			  (when
//			    (for-struct addr (eq @addressLine1 "nowhere"))
//			    (for-struct person (eq @name "Salaboy") )
//			    (for-int age (eq $$ > 10) )
//			  )
func _when(evalCtx *gendsl.EvalCtx, args []gendsl.Expr, options map[string]gendsl.Value) (gendsl.Value, error) {
	optFn := func(p *grete.Production) error {
		env := typeEnv.Clone().WithString("$$", types.FieldSelf)
		for _, arg := range args {
			v, err := arg.EvalWithEnv(env)
			if err != nil {
				return err
			}
			alias, ok := v.Unwrap().(rete.AliasDeclaration)
			if !ok {
				panic("expecting an AliasDeclaration in when")
			}
			p.When = append(p.When, alias)
			env = env.WithUserData("$"+string(alias.Alias), &gendsl.UserData{V: &selectorGenerator{ID: alias.Alias}})
		}
		return nil
	}

	return &gendsl.UserData{V: productionOpt(optFn)}, nil
}

// type AliasDeclaration struct {
// 	Alias     GVIdentity
// 	Type      TypeInfo
// 	Guards    []Guard    // constant test on this alias
// 	JoinTests []JoinTest // join test on this alias and others
// 	NJoinTest []JoinTest // negative join
// }
//

func dslValueToGValue(val gendsl.Value) types.GValue {
	switch val.Type() {
	case gendsl.ValueTypeBool:
		return types.GVBool(val.Unwrap().(bool))
	case gendsl.ValueTypeInt:
		return types.GVInt(val.Unwrap().(int64))
	case gendsl.ValueTypeUInt:
		return types.GVInt(val.Unwrap().(uint64))
	case gendsl.ValueTypeString:
		return types.GVString(val.Unwrap().(string))
	case gendsl.ValueTypeFloat:
		return types.GVFloat(val.Unwrap().(float64))
	case gendsl.ValueTypeNil:
		return &types.GVNil{}
	}
	panic("unsupported type " + val.Type().String())
}
