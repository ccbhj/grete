package dsl

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

type ParseContext struct {
	p   *PRD
	env map[string]any
}

type (
	OptionList map[string]any

	Definition struct {
		DefType string
		ID      string
		Options OptionList
		Body    []any
	}

	Expression struct {
		Op      string
		Operand []any
	}
)

var parserTab map[pegRule]func(*ParseContext, *node32) (any, error)

func init() {
	parserTab = map[pegRule]func(*ParseContext, *node32) (any, error){
		ruleDefinitions:    parseDefinitions,
		ruleDefinition:     parseDefinition,
		ruleDefinitionBody: parseDefinitionBody,
		ruleOptionList:     parseOptionList,
		ruleExpression:     parseExpression,
		ruleIdentifier:     parseIdentifier,
		ruleOperator:       parseChild,
		ruleLiteral:        parseChild,
		ruleBoolLiteral:    parseBoolLiteral,
		ruleFloatLiteral:   parseFloatLiteral,
		ruleIntegerLiteral: parseIntegerLiteral,
		ruleStringLiteral:  parseStringLiteral,
		ruleKeyword:        parseNodeText,
		ruleDefType:        parseNodeText,
	}
}

func MakeParseContext(script string) (*ParseContext, error) {
	parser := &PRD{
		Buffer: script,
		Pretty: true,
	}
	if err := parser.Init(); err != nil {
		return nil, err
	}
	if err := parser.Parse(); err != nil {
		return nil, err
	}
	return &ParseContext{
		p:   parser,
		env: make(map[string]any),
	}, nil
}

func (c *ParseContext) Run() ([]*Definition, error) {
	ast := c.p.AST()
	v, err := c.parseNode(ast)
	if err != nil {
		return nil, err
	}
	return v.([]*Definition), nil
}

func (c *ParseContext) PrintTree() {
	c.p.PrintSyntaxTree()
}

func (c *ParseContext) parseNode(node *node32) (any, error) {
	parser := parserTab[node.pegRule]
	if parser == nil {
		panic("parser for rule " + node.pegRule.String() + " not found")
	}
	return parser(c, node)
}

func parseDefinitions(c *ParseContext, node *node32) (any, error) {
	defs := make([]*Definition, 0, 2)
	i := 0
	node = node.up
	for cur := node; cur != nil; cur = node.next {
		v, err := c.parseNode(cur)
		if err != nil {
			return nil, err
		}
		def, ok := v.(*Definition)
		if !ok {
			return nil, err
		}
		defs = append(defs, def)
		i++
	}

	return defs, nil
}

func parseDefinition(c *ParseContext, node *node32) (any, error) {
	var (
		ok      bool
		id      string
		defType string
		defBody []any
		options map[string]any
	)
	for cur := node.up; cur != nil; cur = cur.next {
		switch cur.pegRule {
		case ruleLPAR, ruleRPAR, ruleLBRK, ruleRBRK:
			continue
		}
		v, err := c.parseNode(cur)
		if err != nil {
			return nil, errors.WithMessage(err, "expecting a DefType")
		}

		rule := cur.pegRule
		switch rule {
		case ruleDefType:
			defType, ok = v.(string)
			if !ok {
				return nil, errors.Errorf("fail to parse %s, expecting a string but got %v", rule, v)
			}
		case ruleIdentifier:
			id, ok = v.(string)
			if !ok {
				return nil, errors.Errorf("fail to parse %s, expecting a string but got %v", rule, v)
			}
			if _, in := c.env[id]; in {
				return nil, SyntaxErrorf(c, cur, "re-define identifier %q is not allowed", id)
			}
		case ruleOptionList:
			options, ok = v.(OptionList)
			if !ok {
				return nil, errors.Errorf("fail to parse %s, expecting a OptionList but got %v", rule, v)
			}
		case ruleDefinitionBody:
			defBody, ok = v.([]any)
			if !ok {
				return nil, errors.Errorf("fail to parse %s", rule)
			}
		default:
			// should never got here
			return nil, SyntaxErrorf(c, cur, "invalid syntax")
		}
	}

	def := &Definition{
		ID:      id,
		DefType: defType,
		Options: options,
		Body:    defBody,
	}
	// TODO: convert def into specify value
	c.env[def.ID] = def

	return def, nil
}

func parseDefinitionBody(c *ParseContext, node *node32) (any, error) {
	defBody := make([]any, 0, 2)
	for cur := node.up; cur != nil; cur = cur.next {
		switch cur.pegRule {
		case ruleLPAR, ruleRPAR, ruleLBRK, ruleRBRK:
			continue
		}
		v, err := c.parseNode(cur)
		if err != nil {
			return nil, errors.WithMessage(err, "expecting a DefType")
		}
		defBody = append(defBody, v)
	}

	return defBody, nil
}

func parseExpression(c *ParseContext, node *node32) (any, error) {
	var (
		operator string
		operands []any
	)
	cur := node.up

	// ignore the LBRK/LPAR
	cur = cur.next
	if cur.pegRule != ruleOperator {
		return nil, SyntaxErrorf(c, cur, "expecting an operator in expression, but got %s", cur.pegRule)
	}
	v, err := c.parseNode(cur)
	if err != nil {
		return nil, err
	}
	operator = v.(string)

	// parse operands
	for cur = cur.up; cur != nil; cur = cur.next {
		v, err := c.parseNode(cur)
		if err != nil {
			return nil, err
		}
		operands = append(operands, v)
	}

	return &Expression{
		Op:      operator,
		Operand: operands,
	}, nil
}

func parseOptionList(c *ParseContext, node *node32) (any, error) {
	opts := make(OptionList, 2)
	for node = node.up; node != nil; node = node.next {
		optID, optVal, err := parseOption(c, node)
		if err != nil {
			return nil, err
		}
		opts[optID] = optVal
	}
	return opts, nil
}

func parseOption(c *ParseContext, node *node32) (string, any, error) {
	var (
		id  string
		val any
	)

	node = node.up
	idVal, err := c.parseNode(node)
	if err != nil {
		return "", nil, errors.Errorf("fail to parse option id(%q)", c.nodeText(node))
	}
	id = idVal.(string)

	val, err = c.parseNode(node.next)
	if err != nil {
		return "", nil, errors.Errorf("fail to parse option val(%q) for option #:%s",
			c.nodeText(node.next), idVal)
	}
	return id, val, nil
}

func parseIdentifier(c *ParseContext, node *node32) (any, error) {
	id, err := c.readChars(node.up)
	if err != nil {
		return nil, err
	}
	return id, nil
}

func parseStringLiteral(c *ParseContext, node *node32) (any, error) {
	unquoted, err := strconv.Unquote(c.nodeText(node))
	if err != nil {
		return nil, err
	}
	return unquoted, nil
}

func parseFloatLiteral(c *ParseContext, node *node32) (any, error) {
	return strconv.ParseFloat(c.nodeText(node), 64)
}

func parseBoolLiteral(c *ParseContext, node *node32) (any, error) {
	s := c.nodeText(node)
	if s == "#t" {
		return true, nil
	}
	return false, nil
}

func parseIntegerLiteral(c *ParseContext, node *node32) (any, error) {
	text := c.nodeText(node)
	suffix := text[len(text)-1]
	switch suffix {
	case 'f', 'F':
		return strconv.ParseFloat(text[:len(text)-1], 64)
	case 'u', 'U':
		return strconv.ParseUint(text[:len(text)-1], 10, 64)
	default:
		return strconv.ParseInt(text, 10, 64)
	}
}

func (c *ParseContext) nodeText(n *node32) string {
	return strings.TrimSpace(string(c.p.buffer[n.begin:n.end]))
}

func (c *ParseContext) readChars(node *node32) (string, error) {
	buf := &strings.Builder{}
	for node.next != nil {
		switch node.pegRule {
		case ruleLetterOrDigit, ruleLetter:
			buf.WriteString(c.nodeText(node))
		default:
			return "", SyntaxErrorf(c, node, "expecting a digit or char")
		}
		node = node.next
	}

	return buf.String(), nil
}

// parse whatever its child is
func parseChild(c *ParseContext, node *node32) (any, error) {
	return c.parseNode(node.up)
}

// parseNodeText return the text of the token node
func parseNodeText(c *ParseContext, node *node32) (any, error) {
	return c.nodeText(node), nil
}

func (r pegRule) String() string {
	return rul3s[r]
}

type SyntaxError struct {
	beginLine, endLine int
	beginSym, endSym   int
	cause              error
}

func (s *SyntaxError) Error() string {
	return fmt.Sprintf("invalid syntax from line %d, symbol %d to line %d, symbol %d: %s",
		s.beginLine, s.beginSym, s.endLine, s.endSym, s.cause)
}

func (s *SyntaxError) Unwrap() error {
	return s.cause
}

func (s *SyntaxError) Cause() error {
	return s.cause
}

func SyntaxErrorf(c *ParseContext, node *node32, f string, args ...any) error {
	pos := translatePositions(c.p.buffer, []int{int(node.begin), int(node.end)})
	beg, end := pos[int(node.begin)], pos[int(node.end)]

	return &SyntaxError{
		beginLine: beg.line,
		endLine:   end.line,
		beginSym:  beg.symbol,
		endSym:    end.symbol,
		cause:     errors.Errorf(f, args...),
	}
}
