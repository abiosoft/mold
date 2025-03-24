package mold

import (
	"errors"
	"fmt"
	"html/template"
	"text/template/parse"
)

// processTree traverses the node tree and swaps render and partial declarations with equivalent template calls.
// It returns all referenced templates encountered during the traversal.
func processTree(t *template.Template, raw string) ([]templateName, error) {
	ts, err := processNode(t.Tree, nil, 0, t.Tree.Root)
	if err != nil {
		if err, ok := err.(posErr); ok {
			line, col := pos(raw, err.pos)
			return ts, fmt.Errorf("%s:%d:%d: %w", t.Name(), line, col, err)
		}
	}

	return ts, nil
}

func processNode(tree *parse.Tree, parent *parse.ListNode, index int, node parse.Node) (ts []templateName, err error) {
	// appendResult appends the specified templates to the list of template names when there are no errors
	appendResult := func(t []templateName, err1 error) {
		if err1 != nil {
			err = err1
		}
		if err == nil {
			ts = append(ts, t...)
		}
	}

	if a, ok := node.(*parse.ActionNode); ok {
		if len(a.Pipe.Cmds) > 0 {
			funcName, tname, _ := getActionArgs(a.Pipe.Cmds[0])
			if err := processActionNode(tree, parent, index, node, funcName); err != nil {
				return ts, err
			}
			if funcName == partialFunc.String() && tname != "" {
				ts = append(ts, templateName{name: tname, typ: partialFunc})
			} else if funcName == renderFunc.String() && tname != "" {
				ts = append(ts, templateName{name: tname, typ: renderFunc})
			}
		}
	}

	if w, ok := node.(*parse.WithNode); ok && w != nil {
		appendResult(processNode(tree, parent, index, w.List))
		appendResult(processNode(tree, parent, index, w.ElseList))
	}
	if l, ok := node.(*parse.ListNode); ok && l != nil {
		for i, n := range l.Nodes {
			appendResult(processNode(tree, l, i, n))
		}
	}
	if i, ok := node.(*parse.IfNode); ok && i != nil {
		appendResult(processNode(tree, parent, index, i.List))
		appendResult(processNode(tree, parent, index, i.ElseList))
	}
	if r, ok := node.(*parse.RangeNode); ok && r != nil {
		appendResult(processNode(tree, parent, index, r.List))
		appendResult(processNode(tree, parent, index, r.ElseList))
	}

	return ts, err
}

func processActionNode(tree *parse.Tree, parent *parse.ListNode, index int, node parse.Node, funcName string) error {
	if parent == nil {
		// this should never happen
		return errors.New("processActionNode error: parent node is nil")
	}

	actionNode := node.(*parse.ActionNode)
	cmd := actionNode.Pipe.Cmds[0]
	_, name, field := getActionArgs(cmd)

	if name == tree.ParseName {
		return posErr{pos: int(actionNode.Pos), message: fmt.Sprintf(`cyclic reference for '%s'`, name)}
	}

	var arg parse.Node = &parse.DotNode{}

	// only handle if the function name is render or partial
	switch {
	case funcName == partialFunc.String():
		if field != nil {
			arg = field
		}
		if name == "" {
			return posErr{pos: int(actionNode.Pos), message: `partial: path to partial file is not specified`}
		}
	case funcName == renderFunc.String():
		if name == "" {
			name = "body"
		}
	default:
		return nil
	}

	cmd.Args = []parse.Node{arg}
	actionNode.Pipe.Cmds = []*parse.CommandNode{cmd}

	tn := &parse.TemplateNode{
		NodeType: parse.NodeTemplate,
		Pos:      actionNode.Pos,
		Line:     actionNode.Line,
		Name:     name,
		Pipe:     actionNode.Pipe,
	}

	// replace the ActionNode with a TemplateNode.
	parent.Nodes[index] = tn
	return nil
}

func getActionArgs(cmd *parse.CommandNode) (fn, file string, field *parse.FieldNode) {
	if len(cmd.Args) > 0 {
		if i, ok := cmd.Args[0].(*parse.IdentifierNode); ok {
			fn = i.Ident
		}
	}
	if len(cmd.Args) > 1 {
		if s, ok := cmd.Args[1].(*parse.StringNode); ok {
			file = s.Text
		}
	}
	if len(cmd.Args) > 2 {
		if f, ok := cmd.Args[2].(*parse.FieldNode); ok {
			field = f
		}
	}
	return
}

// posErr tracks the position in the template file when a parse error occurs.
type posErr struct {
	pos     int
	message string
}

func (p posErr) Error() string {
	return p.message
}

func pos(body string, pos int) (line int, col int) {
	line = 1
	col = 1
	for i, char := range body {
		if i >= pos {
			break
		}

		if char == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}
	return line, col
}

type templateFunc string

func (t templateFunc) String() string { return string(t) }

const (
	renderFunc  templateFunc = "render"
	partialFunc templateFunc = "partial"
)

type templateName struct {
	name string
	typ  templateFunc
}
