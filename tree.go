package mold

import (
	"errors"
	"fmt"
	"html/template"
	"text/template/parse"
)

// processTree traverses the node tree and swaps render and partial declarations with equivalent template calls.
// It returns all referenced templates encountered during the traversal.
func processTree(t *template.Template, raw string, render, partial bool) ([]string, error) {
	ts, err := processNode(nil, 0, t.Tree.Root, nil, render, partial)
	if err != nil {
		if err, ok := err.(posErr); ok {
			line, col := pos(raw, err.pos)
			return ts, fmt.Errorf("%s:%d:%d: %w", t.Name(), line, col, err)
		}
	}

	return ts, nil
}

func processNode(
	parent *parse.ListNode,
	index int,
	node parse.Node,
	parentErr error,
	render,
	partial bool,
) (ts []string, err error) {
	// quit early if an error occured in the parent recursive call
	if parentErr != nil {
		return ts, parentErr
	}

	// appendResult appends the specified templates to the list of template names when there are no errors
	appendResult := func(t []string, err1 error) {
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
			if funcName == renderFunc || funcName == partialFunc {
				if err := processActionNode(parent, index, node, render, partial); err != nil {
					return ts, err
				}
			}
			if tname != "" {
				ts = append(ts, tname)
			}
		}
	}

	if w, ok := node.(*parse.WithNode); ok && w != nil {
		appendResult(processNode(parent, index, w.List, err, render, partial))
		appendResult(processNode(parent, index, w.ElseList, err, render, partial))
	}
	if l, ok := node.(*parse.ListNode); ok && l != nil {
		for i, n := range l.Nodes {
			appendResult(processNode(l, i, n, err, render, partial))
		}
	}
	if i, ok := node.(*parse.IfNode); ok && i != nil {
		appendResult(processNode(parent, index, i.List, err, render, partial))
		appendResult(processNode(parent, index, i.ElseList, err, render, partial))
	}
	if r, ok := node.(*parse.RangeNode); ok && r != nil {
		appendResult(processNode(parent, index, r.List, err, render, partial))
		appendResult(processNode(parent, index, r.ElseList, err, render, partial))
	}

	return ts, err
}

func processActionNode(parent *parse.ListNode, index int, node parse.Node, render, partial bool) error {
	if parent == nil {
		// this should never happen
		return errors.New("processActionNode error: parent node is nil")
	}

	actionNode := node.(*parse.ActionNode)
	cmd := actionNode.Pipe.Cmds[0]
	funcName, name, field := getActionArgs(cmd)

	var arg parse.Node = &parse.DotNode{}

	// only handle if the function name is render or partial
	switch {
	case funcName == partialFunc && partial:
		if field != nil {
			arg = field
		}
		if name == "" {
			return posErr{pos: int(actionNode.Pos), message: `partial: path to partial file is not specified`}
		}
	case funcName == renderFunc && render:
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

// posErr tracks the position in the layout file when a parse error occurs.
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
