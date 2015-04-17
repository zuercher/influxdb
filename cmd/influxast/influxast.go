package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/influxdb/influxdb/influxql"
)

var query = flag.String("q", "", "query to parse")
var expr = flag.String("e", "", "expression to parse")
var reduce = flag.Bool("reduce", false, "reduces expressions")
var print = flag.Bool("print", false, "prints AST string representation")
var dot = flag.String("dot", "", "writes AST in dot file format to the specified file")
var svg = flag.String("svg", "", "writes AST as an svg image to the specified file (requires graphviz)")

// type Parser interface {
// 	Parse(s string) (influxql.Node, error)
// }

// type queryParser struct {}

// func (queryParser) Parse(s string) (influxql.Node, error) {
// 	return influxql.ParseQuery(s)
// }

// type exprParser struct {}

// func (exprParser) Parse(s string) (influxql.Node, error) {
// 	return influxql.ParseExpr(s)
// }

func main() {
	flag.Parse()

	if *query == "" && *expr == "" {
		flag.Usage()
		os.Exit(1)
	}

	var err error
	var ast influxql.Node
	var input string
	//var parser Parser

	if *query != "" {
		input = *expr
		ast, err = influxql.ParseQuery(input)
	} else if *expr != "" {
		input = *expr
		ast, err = influxql.ParseExpr(input)
	}
	check(err)

	if *reduce {
		now := time.Now().UTC()
		err = reduceExprs(ast, &influxql.NowValuer{Now: now})
		check(err)
	}

	var dotFile *os.File

	if *dot != "" {
		dotFile, err = os.Create(*dot)
		check(err)
		defer dotFile.Close()
	}

	if *print {
		fmt.Printf("ast = %s\n", ast.String())
	}

	if dotFile != nil {
		ast2dot(dotFile, input, ast)
	}

	if *svg != "" {
		cmd := exec.Command("dot", "-Tsvg", "-o", *svg)
		dotStdin, err := cmd.StdinPipe()
		check(err)
		check(cmd.Start())
		ast2dot(dotStdin, input, ast)
		dotStdin.Close()
		check(cmd.Wait())
	}
}

func ast2dot(w io.Writer, title string, ast influxql.Node) {
	_, err := fmt.Fprintln(w, "digraph AST {")
	check(err)

	fmt.Fprintf(w, "\tranksep=1.0;\n")
	fmt.Fprintf(w, "\tlabelloc=\"t\"\n")
	fmt.Fprintf(w, "\tlabel=\"%s\"\n", title)
	fmt.Fprintf(w, "\tnode [shape=oval]\n")

	//astDot := newDotNode(ast)
	//ast2dotWalk(w, astDot, nil)
	influxql.Walk(newAst2DotVisitor(w), ast, nil)

	fmt.Fprintln(w, "}")
}

var nextNodeID int64 = 0

func newNodeName() string {
	n := fmt.Sprintf("n%d", nextNodeID)
	nextNodeID++
	return n
}

type dotNode struct {
	Name string
	Node influxql.Node
}

func newDotNode(n influxql.Node) *dotNode {
	return &dotNode{Name: newNodeName(), Node: n}
}

func (dn *dotNode) String() string {
	var buf bytes.Buffer
	buf.WriteString(dn.Name)
	label := dn.Label()
	if label != "" {
		buf.WriteString(` [label="`)
		buf.WriteString(label)
		buf.WriteString(`"]`)
	}
	return buf.String()
}

func (dn *dotNode) Label() string {
	switch n := dn.Node.(type) {
	case *influxql.BinaryExpr:
		return n.Op.String()
	case *influxql.VarRef:
		return n.Val
	case *influxql.StringLiteral:
		return fmt.Sprintf(`'%s'`, n.Val)
	case *influxql.DurationLiteral:
		return n.Val.String()
	case *influxql.TimeLiteral:
		return fmt.Sprintf(`'%s'`, n.Val.String())
	case *influxql.Call:
		return n.String()
	case *influxql.RegexLiteral:
		return fmt.Sprintf(`/%s/`, n.Val.String())
	case *influxql.NumberLiteral:
		return strconv.FormatFloat(n.Val, 'f', -1, 64)
	case *influxql.ParenExpr:
		return "(<expr>)"
	case *influxql.Query:
		return "<query>"
	case *influxql.SelectStatement:
		return "SELECT"
	}
	return ""
}

type ast2dotVisitor struct {
	w     io.Writer
	nodes map[influxql.Node]*dotNode
}

func newAst2DotVisitor(w io.Writer) *ast2dotVisitor {
	return &ast2dotVisitor{
		w:     w,
		nodes: map[influxql.Node]*dotNode{},
	}
}

func (v *ast2dotVisitor) Visit(node, parent influxql.Node) influxql.Visitor {
	p, ok := v.nodes[parent]
	if !ok {
		p = newDotNode(parent)
		v.nodes[parent] = p
	}

	n := newDotNode(node)
	v.nodes[node] = n

	printDot(v.w, n, p)

	return v
}

// reduceExprs walks the AST and reduces expressions.
func reduceExprs(ast influxql.Node, v influxql.Valuer) error {
	influxql.WalkFunc(ast, nil, func(node, parent influxql.Node) {
		switch n := node.(type) {
		case influxql.Fields:
			for _, f := range n {
				f.Expr = influxql.Reduce(f.Expr, v)
			}
		}
	})

	return nil
}

// func ast2dotWalk(w io.Writer, node, parent *dotNode) {
// 	switch n := node.Node.(type) {
// 	case *influxql.BinaryExpr:
// 		node.Label = n.Op.String()
// 		printDot(w, node, parent)
// 		ast2dotWalk(w, newDotNode(n.LHS), node)
// 		ast2dotWalk(w, newDotNode(n.RHS), node)
// 	case *influxql.VarRef:
// 		node.Label = n.Val
// 		printDot(w, node, parent)
// 	case *influxql.StringLiteral:
// 		node.Label = fmt.Sprintf(`'%s'`, n.Val)
// 		printDot(w, node, parent)
// 	case *influxql.DurationLiteral:
// 		node.Label = n.Val.String()
// 		printDot(w, node, parent)
// 	case *influxql.TimeLiteral:
// 		node.Label = fmt.Sprintf(`'%s'`, n.Val.String())
// 		printDot(w, node, parent)
// 	case *influxql.Call:
// 		node.Label = n.String()
// 		printDot(w, node, parent)
// 	case *influxql.RegexLiteral:
// 		node.Label = fmt.Sprintf(`/%s/`, n.Val.String())
// 		printDot(w, node, parent)
// 	case *influxql.NumberLiteral:
// 		node.Label = strconv.FormatFloat(n.Val, 'f', -1, 64)
// 		printDot(w, node, parent)
// 	case *influxql.ParenExpr:
// 		node.Label = "(<expr>)"
// 		printDot(w, node, parent)
// 		ast2dotWalk(w, newDotNode(n.Expr), node)
// 	case *influxql.Query:
// 		query2dot(w, n, parent)
// 	case *influxql.SelectStatement:
// 		select2dot(w, n, parent)
// 	}
// }

// func query2dot(w io.Writer, q *influxql.Query, parent *dotNode) {
// 	node := newDotNode(q)
// 	node.Label = "query"
// 	printDot(w, node, parent)
// }

// func select2dot(w io.Writer, s *influxql.SelectStatement, parent *dotNode) {

// }

func printDot(w io.Writer, node, parent *dotNode) {
	printDotNode(w, node)
	if parent.Node != nil {
		printDotEdge(w, parent, node)
	}
}

func printDotNode(w io.Writer, node *dotNode) {
	fmt.Fprintf(w, "\t%s\n", node)
}

func printDotEdge(w io.Writer, parent, child *dotNode) {
	if parent == nil || child == nil {
		return
	}
	fmt.Fprintf(w, "\t%s -> %s\n", parent.Name, child.Name)
}

func check(err error) {
	if err != nil {
		fmt.Printf("error: %s\n", err.Error())
		os.Exit(1)
	}
}
