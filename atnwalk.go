package atnwalk

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/antlr/antlr4/runtime/Go/antlr/v4"
)

type TreeNode interface {
	GetParent() TreeNode
	GetChildren() []TreeNode
}

type RuleNode struct {
	Parent     TreeNode
	Children   []TreeNode
	StartState *antlr.RuleStartState
}

func (n *RuleNode) GetParent() TreeNode {
	return n.Parent
}

func (n *RuleNode) GetChildren() []TreeNode {
	return n.Children
}

func NewRuleNode(parent TreeNode, startState *antlr.RuleStartState) *RuleNode {
	return &RuleNode{Parent: parent, Children: []TreeNode{}, StartState: startState}
}

type SymbolNode struct {
	Parent     TreeNode
	Children   []TreeNode
	StartState *antlr.RuleStartState
}

func (n *SymbolNode) GetParent() TreeNode {
	return n.Parent
}

func (n *SymbolNode) GetChildren() []TreeNode {
	return n.Children
}

func NewSymbolNode(parent TreeNode, startState *antlr.RuleStartState) *SymbolNode {
	return &SymbolNode{Parent: parent, Children: []TreeNode{}, StartState: startState}
}

type LiteralNode struct {
	Parent TreeNode
	Text   rune
}

func (n *LiteralNode) GetParent() TreeNode {
	return n.Parent
}

func (n *LiteralNode) GetChildren() []TreeNode {
	return []TreeNode{}
}

func NewLiteralNode(parent TreeNode, text rune) *LiteralNode {
	return &LiteralNode{Parent: parent, Text: text}
}

type ATNWalker struct {
	Lexer         antlr.Lexer
	Parser        antlr.Parser
	parserRouter  map[int]*Router
	lexerRouter   map[int]*Router
	deadline      time.Time
	deadlineIsSet bool
}

func NewATNWalker(parser antlr.Parser, lexer antlr.Lexer) *ATNWalker {
	walker := &ATNWalker{Lexer: lexer, Parser: parser, parserRouter: make(map[int]*Router), lexerRouter: make(map[int]*Router), deadlineIsSet: false}
	return walker
}

func (w *ATNWalker) SetDeadline(t time.Time) {
	w.deadline = t
	w.deadlineIsSet = true
}

type ParserTraceEdge struct {
	State  antlr.ATNState
	Choice int
	Cursor int
}

func (w *ATNWalker) encodeParserRuleATN(encoder *Encoder, node *WrappedTreeNode) {

	var transition antlr.Transition
	cursor := 0
	choice := 0
	traceStack := &Stack[*ParserTraceEdge]{}
	var state antlr.ATNState
	state = w.Parser.GetATN().GetRuleIndexToStartStateSlice()[node.GetRuleIndex()]
	for !(state.GetStateType() == antlr.ATNStateRuleStop && cursor == len(node.Children)) {
		// backtrack
		if choice >= len(state.GetTransitions()) || state.GetStateType() == antlr.ATNStateRuleStop {
			t := traceStack.Pop()
			state = t.State
			choice = t.Choice + 1
			cursor = t.Cursor
			continue
		}

		transition = state.GetTransitions()[choice]
		switch t := transition.(type) {
		case *antlr.RuleTransition:
			if cursor < len(node.Children) {
				if ruleCtx, ok := node.Children[cursor].OriginalNode.(antlr.ParserRuleContext); ok {
					if ruleCtx.GetRuleIndex() == t.GetRuleIndex() {
						traceStack.Push(&ParserTraceEdge{state, choice, cursor})
						state = t.GetFollowState()
						cursor++
						choice = 0
						continue
					}
				}
			}
		case *antlr.AtomTransition:
			if cursor < len(node.Children) {
				if terminalNode, ok := node.Children[cursor].OriginalNode.(antlr.TerminalNode); ok {
					if terminalNode.GetSymbol().GetTokenType() == t.GetLabelValue() {
						traceStack.Push(&ParserTraceEdge{state, choice, cursor})
						state = t.GetTarget()
						cursor++
						choice = 0
						continue
					}
				}
			}
		case *antlr.SetTransition:
			if cursor < len(node.Children) {
				if terminal, ok := node.Children[cursor].OriginalNode.(antlr.TerminalNode); ok {
					if t.GetLabel().Contains(terminal.GetSymbol().GetTokenType()) {
						traceStack.Push(&ParserTraceEdge{state, choice, cursor})
						state = t.GetTarget()
						cursor++
						choice = 0
						continue
					}
				}
			}
		default:
			traceStack.Push(&ParserTraceEdge{state, choice, cursor})
			state = transition.(antlr.AnyTransition).GetTarget()
			choice = 0
			continue
		}
		// try next choice from the current state
		choice++
	}

	edges := make([]*ParserTraceEdge, traceStack.Size())
	for i := traceStack.Size() - 1; i >= 0; i-- {
		edges[i] = traceStack.Pop()
	}
	headerSet := false
	for _, edge := range edges {
		if len(edge.State.GetTransitions()) > 1 {
			if !headerSet {
				encoder.WriteRuleHeader(node.GetRuleIndex(), len(w.Parser.GetATN().GetRuleIndexToStartStateSlice()), false)
				headerSet = true
			}
			encoder.Encode(edge.Choice, len(edge.State.GetTransitions()))
		}

		transition := edge.State.GetTransitions()[edge.Choice]
		switch t := transition.(type) {
		case *antlr.SetTransition:
			if t.GetLabel().Length() > 1 {
				if !headerSet {
					encoder.WriteRuleHeader(node.GetRuleIndex(), len(w.Parser.GetATN().GetRuleIndexToStartStateSlice()), false)
					headerSet = true
				}
				encoder.Encode(t.GetLabel().GetIndex(node.Children[edge.Cursor].OriginalNode.(antlr.TerminalNode).GetSymbol().GetTokenType()), t.GetLabel().Length())
			}
		}
	}
}

// to avoid memory leaks
func cleanupTraces(traceNodes ...*LexerTrace) {
	stack := &Stack[*LexerTrace]{}
	for _, t := range traceNodes {
		stack.Push(t)
	}
	var node *LexerTrace
	for !stack.IsEmpty() {
		node = stack.Pop()
		if node.SubTraces != nil {
			for _, st := range node.SubTraces {
				stack.Push(st)
			}
		}
		node.SubTraces = nil
	}
}

func computeRuleMatches(text string, cursor int, state antlr.ATNState) *Stack[*LexerTrace] {
	semaphore := make(chan struct{}, runtime.NumCPU())
	for i := 0; i < runtime.NumCPU(); i++ {
		semaphore <- struct{}{}
	}
	results := make(chan *LexerTrace, runtime.NumCPU())
	for i := len(text); i > cursor; i-- {
		go func(i int) {
			<-semaphore
			results <- match(text[cursor:i], state)
			semaphore <- struct{}{}
		}(i)
	}
	stack := &Stack[*LexerTrace]{}
	for i := len(text); i > cursor; i-- {
		solution := <-results
		if solution != nil {
			stack.Push(solution)
		}
	}
	return stack
}

func match(text string, state antlr.ATNState) *LexerTrace {
	// return valid trace if we can match the text, otherwise return nil

	var transition antlr.Transition
	cursor := 0
	choice := 0
	traceStack := &Stack[*LexerTraceEdge]{}

	ruleMatches := map[string]*Stack[*LexerTrace]{}

	var subTraces []*LexerTrace
	for !(state.GetStateType() == antlr.ATNStateRuleStop && cursor == len(text)) {
		// backtrack or report mismatch
		if choice >= len(state.GetTransitions()) && len(state.GetTransitions()) > 0 || state.GetStateType() == antlr.ATNStateRuleStop {
			// report mismatch
			if traceStack.IsEmpty() {
				cleanupTraces(subTraces...)
				for i := 0; i < len(subTraces); i++ {
					subTraces[i] = nil
				}
				return nil
			}
			// backtrack
			t := traceStack.Pop()
			state = t.State
			choice = t.Choice + 1
			cursor = t.Cursor

			// special case, when other token matches could be possible
			// do not continue with the next choice then, but with the next match
			id := id(t.State.GetStateNumber(), t.Choice, cursor)
			if possibleMatches, ok := ruleMatches[id]; ok {
				if !possibleMatches.IsEmpty() {
					choice = t.Choice
				}
			}

			cleanupTraces(subTraces[t.SubTraceLen:]...)
			for i := len(subTraces) - 1; i >= t.SubTraceLen; i-- {
				subTraces[i] = nil
			}
			subTraces = subTraces[:t.SubTraceLen]
			continue
		}

		transition = state.GetTransitions()[choice]
		switch t := transition.(type) {
		case *antlr.RuleTransition:
			if cursor < len(text) {

				id := id(state.GetStateNumber(), choice, cursor)
				possibleMatches, ok := ruleMatches[id]
				if !ok {
					possibleMatches = computeRuleMatches(text, cursor, transition.(antlr.AnyTransition).GetTarget())
					ruleMatches[id] = possibleMatches
				}

				if !possibleMatches.IsEmpty() {
					trace := possibleMatches.Pop()
					traceStack.Push(&LexerTraceEdge{state, choice, cursor, len(subTraces)})
					subTraces = append(subTraces, trace)
					state = t.GetFollowState()
					cursor += len(trace.Text)
					choice = 0
					continue
				}
			}
		case *antlr.AtomTransition:
			if cursor < len(text) {
				if rune(text[cursor]) == rune(t.GetLabelValue()) {
					traceStack.Push(&LexerTraceEdge{state, choice, cursor, len(subTraces)})
					state = transition.(antlr.AnyTransition).GetTarget()
					cursor++
					choice = 0
					continue
				}
			}
		case *antlr.NotSetTransition:
			if cursor < len(text) {
				possibleRunes := t.GetLabel().Complement()
				if possibleRunes.Contains(int(rune(text[cursor]))) {
					traceStack.Push(&LexerTraceEdge{state, choice, cursor, len(subTraces)})
					state = transition.(antlr.AnyTransition).GetTarget()
					cursor++
					choice = 0
					continue
				}
			}
		case *antlr.SetTransition:
			if cursor < len(text) {
				possibleRunes := t.GetLabel()
				if possibleRunes.Contains(int(rune(text[cursor]))) {
					traceStack.Push(&LexerTraceEdge{state, choice, cursor, len(subTraces)})
					state = transition.(antlr.AnyTransition).GetTarget()
					cursor++
					choice = 0
					continue
				}
			}
		case *antlr.RangeTransition:
			if cursor < len(text) {
				possibleRunes := t.GetLabel()
				if possibleRunes.Contains(int(rune(text[cursor]))) {
					traceStack.Push(&LexerTraceEdge{state, choice, cursor, len(subTraces)})
					state = transition.(antlr.AnyTransition).GetTarget()
					cursor++
					choice = 0
					continue
				}
			}
		default:
			traceStack.Push(&LexerTraceEdge{state, choice, cursor, len(subTraces)})
			state = transition.(antlr.AnyTransition).GetTarget()
			choice = 0
			continue
		}
		// try next choice from the current state
		choice++
	}

	edges := make([]*LexerTraceEdge, traceStack.Size())
	for i := traceStack.Size() - 1; i >= 0; i-- {
		edges[i] = traceStack.Pop()
	}

	return &LexerTrace{text, edges, subTraces}
}

type LexerTraceEdge struct {
	State       antlr.ATNState
	Choice      int
	Cursor      int
	SubTraceLen int
}

type LexerTrace struct {
	Text      string
	Edges     []*LexerTraceEdge
	SubTraces []*LexerTrace
}

func id(state, choice, cursor int) string {
	idBytes := make([]byte, 0, 24)
	for _, number := range []int{state, choice, cursor} {
		idBytes = append(idBytes,
			byte(number>>56),
			byte(number&0x00ff000000000000>>48),
			byte(number&0x0000ff0000000000>>40),
			byte(number&0x000000ff00000000>>32),
			byte(number&0x00000000ff000000>>24),
			byte(number&0x0000000000ff0000>>16),
			byte(number&0x000000000000ff00>>8),
			byte(number&0x00000000000000ff))
	}
	return string(idBytes)
}

func (w *ATNWalker) encodeLexerSymbolATN(encoder *Encoder, node antlr.Token) {
	// TODO: currently, we just ignore EOF tokens, in the original SQLite grammar this caused to produce invalid statements <sql_stmt><EOF><sql_stmt> ...
	//       right now, this is a known limitation but a reasonable one since the above example seems odd
	if node.GetTokenType() != antlr.TokenEOF {
		var trace *LexerTrace
		trace = match(node.GetText(), w.Lexer.GetATN().GetRuleIndexToStartStateSlice()[node.GetTokenType()-1])
		nextTracesQueue := &Queue[*LexerTrace]{}
		nextTracesQueue.Enqueue(trace)
		for !nextTracesQueue.IsEmpty() {
			trace = nextTracesQueue.Dequeue()
			headerSet := false
			// encoder.WriteRuleHeader(trace.Edges[0].State.GetRuleIndex(), len(w.Lexer.GetATN().GetRuleIndexToStartStateSlice()), true)
			for _, edge := range trace.Edges {
				if len(edge.State.GetTransitions()) > 1 {
					if !headerSet {
						encoder.WriteRuleHeader(trace.Edges[0].State.GetRuleIndex(), len(w.Lexer.GetATN().GetRuleIndexToStartStateSlice()), true)
						headerSet = true
					}
					encoder.Encode(edge.Choice, len(edge.State.GetTransitions()))
				}
				transition := edge.State.GetTransitions()[edge.Choice]
				switch t := transition.(type) {
				case *antlr.NotSetTransition:
					possibleRunes := t.GetLabel().Complement()
					if possibleRunes.Length() > 1 {
						if !headerSet {
							encoder.WriteRuleHeader(trace.Edges[0].State.GetRuleIndex(), len(w.Lexer.GetATN().GetRuleIndexToStartStateSlice()), true)
							headerSet = true
						}
						encoder.Encode(possibleRunes.GetIndex(int(rune(trace.Text[edge.Cursor]))), possibleRunes.Length())
					}
				case *antlr.SetTransition:
					possibleRunes := t.GetLabel()
					if possibleRunes.Length() > 1 {
						if !headerSet {
							encoder.WriteRuleHeader(trace.Edges[0].State.GetRuleIndex(), len(w.Lexer.GetATN().GetRuleIndexToStartStateSlice()), true)
							headerSet = true
						}
						encoder.Encode(possibleRunes.GetIndex(int(rune(trace.Text[edge.Cursor]))), possibleRunes.Length())
					}
				case *antlr.RangeTransition:
					possibleRunes := t.GetLabel()
					if possibleRunes.Length() > 1 {
						if !headerSet {
							encoder.WriteRuleHeader(trace.Edges[0].State.GetRuleIndex(), len(w.Lexer.GetATN().GetRuleIndexToStartStateSlice()), true)
							headerSet = true
						}
						encoder.Encode(possibleRunes.GetIndex(int(rune(trace.Text[edge.Cursor]))), possibleRunes.Length())
					}
				}
			}
			for _, subTrace := range trace.SubTraces {
				nextTracesQueue.Enqueue(subTrace)
			}
		}
	}
}

type WrappedTreeNode struct {
	OriginalNode antlr.Tree
	Parent       *WrappedTreeNode
	Children     []*WrappedTreeNode
}

func NewWrappedTreeNode(root antlr.Tree) *WrappedTreeNode {
	newRoot := &WrappedTreeNode{OriginalNode: root}
	var node *WrappedTreeNode
	stack := &Stack[*WrappedTreeNode]{}
	stack.Push(newRoot)
	for !stack.IsEmpty() {
		node = stack.Pop()
		node.addChildren(node.OriginalNode.GetChildren()...)
		for _, child := range node.Children {
			stack.Push(child)
		}
	}
	return newRoot
}

func (n *WrappedTreeNode) addChildren(nodes ...antlr.Tree) {
	if cap(n.Children)-len(n.Children) < len(nodes) {
		newChildren := make([]*WrappedTreeNode, len(n.Children), (cap(n.Children)+len(nodes))<<1)
		copy(newChildren, n.Children)
	}
	for _, node := range nodes {
		n.Children = append(n.Children, &WrappedTreeNode{OriginalNode: node, Parent: n})
	}
}

func (n *WrappedTreeNode) IsRule() bool {
	_, ok := n.OriginalNode.(antlr.ParserRuleContext)
	return ok
}

func (n *WrappedTreeNode) IsSymbol() bool {
	_, ok := n.OriginalNode.(antlr.TerminalNode)
	return ok
}
func (n *WrappedTreeNode) IsLeftRecursive() bool {
	if n.IsRule() && n.Parent != nil {
		if n.Parent.IsRule() {
			return n.Parent.GetRuleIndex() == n.GetRuleIndex()
		}
	}
	return false
}

func (n *WrappedTreeNode) IsFirstChild() bool {
	if n.Parent != nil {
		return n == n.Parent.Children[0]
	}
	return false
}

func (n *WrappedTreeNode) GetRuleIndex() int {
	return n.OriginalNode.(antlr.ParserRuleContext).GetRuleIndex()
}

func (w *ATNWalker) wrapANTLRTreeAndEliminateLeftRecursion(root antlr.Tree) *WrappedTreeNode {
	var node, parent, firstChild *WrappedTreeNode
	newRoot := NewWrappedTreeNode(root)
	stack := &Stack[*WrappedTreeNode]{}
	stack.Push(newRoot)
	for !stack.IsEmpty() {
		node = stack.Pop()
		if node.IsSymbol() || len(node.Children) < 1 {
			continue
		}
		parent = node.Parent
		firstChild = node.Children[0]
		if firstChild.IsSymbol() || !firstChild.IsLeftRecursive() {
			if node.IsLeftRecursive() && node.IsFirstChild() {
				for _, child := range node.Children {
					child.Parent = parent
				}
				stack.Push(parent)
				parent.Children = append(node.Children, parent.Children[1:]...)
				continue
			}
			for i := len(node.Children) - 1; i >= 0; i-- {
				stack.Push(node.Children[i])
			}
		} else {
			stack.Push(firstChild)
		}
	}
	return newRoot
}

func (w *ATNWalker) Encode(root antlr.Tree) []byte {
	newRoot := w.wrapANTLRTreeAndEliminateLeftRecursion(root)
	encoder := NewEncoder(nil)
	var node *WrappedTreeNode

	nextNodesStack := &Stack[*WrappedTreeNode]{}
	nextNodesStack.Push(newRoot)
	for !nextNodesStack.IsEmpty() {
		node = nextNodesStack.Pop()
		if node.IsRule() {
			w.encodeParserRuleATN(encoder, node)
			for i := len(node.Children) - 1; i >= 0; i-- {
				nextNodesStack.Push(node.Children[i])
			}
		} else {
			w.encodeLexerSymbolATN(encoder, node.OriginalNode.(antlr.TerminalNode).GetSymbol())
		}
	}

	return encoder.Bytes()
}

func (w *ATNWalker) decodeParserRuleATN(decoder *Decoder, parent *RuleNode) {

	decoder.Init(parent.StartState.GetRuleIndex(), false)

	var state antlr.ATNState = parent.StartState
	var transition antlr.Transition

	// for tracking
	edges := make(chan *RouteEdge, 128)
	okLearned := make(chan bool)
	var rules []int
	var rootPathRules map[int]struct{}

	router, ok := w.parserRouter[parent.StartState.GetRuleIndex()]
	if !ok {
		router = NewRouter(
			state.GetRuleIndex(),
			state.GetStateNumber(),
			state.GetATN().GetRuleIndexToStopStateSlice()[parent.StartState.GetRuleIndex()].GetStateNumber(),
			state.GetATN(),
			decoder)
		w.parserRouter[parent.StartState.GetRuleIndex()] = router
	}
	router.mutex.Lock()
	go router.LearnRoutes(edges, okLearned)
	defer func() {
		go func() {
			edges <- nil
			<-okLearned
			close(edges)
			router.mutex.Unlock()
		}()
	}()

	choice := -1
	prevState, prevChoice := -2, -2
	for state.GetStateType() != antlr.ATNStateRuleStop {

		if w.exceededDeadline() {
			return
		}

		// track visited states where a choice needs to be made, the respective choices,
		// and the ruleIndices encountered for each choice made
		numTransitions := len(state.GetTransitions())
		if numTransitions > 1 {
			if prevState >= 0 {
				edges <- &RouteEdge{prevState, state.GetStateNumber(), prevChoice, rules}
			}
			if !decoder.usePRNG {
				choice = decoder.Decode(numTransitions)
			} else {
				if rootPathRules == nil {
					rootPathRules = make(map[int]struct{})
					p := parent
					for p != nil {
						rootPathRules[p.StartState.BaseATNState.GetRuleIndex()] = struct{}{}
						if p.GetParent() != nil {
							p = p.GetParent().(*RuleNode)
						} else {
							p = nil
						}
					}
				}
				edges <- nil
				<-okLearned
				choice = router.route(state.GetStateNumber(), rootPathRules)
				if decoder.writeBackEncoder != nil {
					if !decoder.writeBackHead.isSet {
						decoder.writeBackEncoder.WriteRuleHeader(decoder.writeBackHead.ruleIndex, decoder.writeBackHead.numRules, decoder.writeBackHead.isLexerRule)
						decoder.writeBackHead.isSet = true
					}
					decoder.writeBackEncoder.Encode(choice, numTransitions)
				}
			}
			prevState = state.GetStateNumber()
			prevChoice = choice
			rules = make([]int, 0)
		} else {
			choice = 0
		}

		transition = state.GetTransitions()[choice]
		switch t := transition.(type) {
		case *antlr.RuleTransition:
			parent.Children = append(parent.Children, NewRuleNode(parent, w.Parser.GetATN().GetRuleIndexToStartStateSlice()[t.GetRuleIndex()]))
			rules = append(rules, t.GetRuleIndex())
			state = t.GetFollowState()
			continue
		case *antlr.AtomTransition:
			// from what I undestood:
			// - the AtomTransition label symbolizes the token type
			// - token types start at 1, a special token is TokenEOF, which is -1
			// - they have a size, for parsers, of len(antlr.BaseParser.SymbolicNames)-1
			// - thus, the token type is also the index of the slice of SymbolicNames
			// - each element in antlr.BaseParser.SymbolicNames, is a rule in the antlr.BaseLexer.RuleNames
			//   (antlr.BaseParser.SymbolicNames is a subset of antlr.BaseLexer.RuleNames, the lexer may have additional rules defined,
			//   which are, however, not present in the parser ATN as tokens in the AtomTransition)
			// - the lexer RuleNames start at 0 and not at 1, thus the (parser transition) label-1 represents the corresponding rule in the lexer
			if t.GetLabelValue() != antlr.TokenEOF {
				parent.Children = append(parent.Children, NewSymbolNode(nil, w.Lexer.GetATN().GetRuleIndexToStartStateSlice()[t.GetLabelValue()-1]))
			}
		case *antlr.SetTransition:
			// from what I understood:
			// - a SetTransition in a parser encodes a set of symbols, i.e. token types
			// - using the same logic as for AtomTransitions to obtain the lexer rule start state should work
			lexerRuleIndex := t.GetLabel().Get(decoder.Decode(t.GetLabel().Length())) - 1
			parent.Children = append(parent.Children, NewSymbolNode(nil, w.Lexer.GetATN().GetRuleIndexToStartStateSlice()[lexerRuleIndex]))
		case *antlr.RangeTransition:
			panic("Transition type antlr.RangeTransition is not implemented for decodeParserRuleATN.")
		}
		state = transition.(antlr.AnyTransition).GetTarget()
	}
	if prevState >= 0 {
		edges <- &RouteEdge{prevState, state.GetStateNumber(), prevChoice, rules}
	}
}

func (w *ATNWalker) decodeLexerSymbolATN(decoder *Decoder, parent *SymbolNode) {

	decoder.Init(parent.StartState.GetRuleIndex(), true)

	var state antlr.ATNState = parent.StartState
	var transition antlr.Transition

	// for tracking
	edges := make(chan *RouteEdge, 128)
	okLearned := make(chan bool)
	var rules []int
	var rootPathRules map[int]struct{}

	router, ok := w.lexerRouter[parent.StartState.GetRuleIndex()]
	if !ok {
		router = NewRouter(
			state.GetRuleIndex(),
			state.GetStateNumber(),
			state.GetATN().GetRuleIndexToStopStateSlice()[state.GetRuleIndex()].GetStateNumber(),
			state.GetATN(),
			decoder)
		w.lexerRouter[parent.StartState.GetRuleIndex()] = router
	}
	router.mutex.Lock()
	go router.LearnRoutes(edges, okLearned)
	defer func() {
		go func() {
			edges <- nil
			<-okLearned
			close(edges)
			router.mutex.Unlock()
		}()
	}()

	// track visited states where a choice needs to be made, the respective choices themselves,
	// and the ruleIndices encountered
	choice := -1
	prevState, prevChoice := -2, -2
	for state.GetStateType() != antlr.ATNStateRuleStop {

		if w.exceededDeadline() {
			return
		}

		numTransitions := len(state.GetTransitions())
		if numTransitions > 1 {
			if prevState >= 0 {
				edges <- &RouteEdge{prevState, state.GetStateNumber(), prevChoice, rules}
			}
			if !decoder.usePRNG {
				choice = decoder.Decode(numTransitions)
			} else {
				if rootPathRules == nil {
					rootPathRules = make(map[int]struct{})
					p := parent
					for p != nil {
						rootPathRules[p.StartState.BaseATNState.GetRuleIndex()] = struct{}{}
						if p.GetParent() != nil {
							p = p.GetParent().(*SymbolNode)
						} else {
							p = nil
						}
					}
				}
				edges <- nil
				<-okLearned
				choice = router.route(state.GetStateNumber(), rootPathRules)
				if decoder.writeBackEncoder != nil {
					if !decoder.writeBackHead.isSet {
						decoder.writeBackEncoder.WriteRuleHeader(decoder.writeBackHead.ruleIndex, decoder.writeBackHead.numRules, decoder.writeBackHead.isLexerRule)
						decoder.writeBackHead.isSet = true
					}
					decoder.writeBackEncoder.Encode(choice, numTransitions)
				}
			}
			prevState = state.GetStateNumber()
			prevChoice = choice
			rules = make([]int, 0)
		} else {
			choice = 0
		}

		transition = state.GetTransitions()[choice]
		switch t := transition.(type) {
		case *antlr.RuleTransition:
			parent.Children = append(parent.Children, NewSymbolNode(parent, w.Lexer.GetATN().GetRuleIndexToStartStateSlice()[t.GetRuleIndex()]))
			state = t.GetFollowState()
			continue
		case *antlr.AtomTransition:
			parent.Children = append(parent.Children, NewLiteralNode(parent, rune(t.GetLabelValue())))
		// order is important here, a NotSetTransition is also a SetTransition so find out whether this is a NotSetTransition first
		case *antlr.NotSetTransition:
			possibleRunes := t.GetLabel().Complement()
			chosenRune := rune(possibleRunes.Get(decoder.Decode(possibleRunes.Length())))
			parent.Children = append(parent.Children, NewLiteralNode(parent, chosenRune))
		case *antlr.SetTransition:
			possibleRunes := t.GetLabel()
			chosenRune := rune(possibleRunes.Get(decoder.Decode(possibleRunes.Length())))
			parent.Children = append(parent.Children, NewLiteralNode(parent, chosenRune))
		case *antlr.RangeTransition:
			possibleRunes := t.GetLabel()
			chosenRune := rune(possibleRunes.Get(decoder.Decode(possibleRunes.Length())))
			parent.Children = append(parent.Children, NewLiteralNode(parent, chosenRune))
			// TODO: why does it crash here?
			//case *antlr.WildcardTransition:
			//	possibleRunes := t.GetLabel()
			//	chosenRune := rune(possibleRunes.Get(decoder.Decode(possibleRunes.Length())))
			//	parent.Children = append(parent.Children, NewLiteralNode(parent, chosenRune))
		}
		state = transition.(antlr.AnyTransition).GetTarget()
	}
	if prevState >= 0 {
		edges <- &RouteEdge{prevState, state.GetStateNumber(), prevChoice, rules}
	}
}

func (w *ATNWalker) printTree(node TreeNode, depth int) {
	fmt.Print(strings.Repeat(" ", depth*6))
	switch n := node.(type) {
	case *RuleNode:
		fmt.Printf("(R) " + w.Parser.GetRuleNames()[n.StartState.GetRuleIndex()] + " [" + fmt.Sprint(n.StartState.GetRuleIndex()) + "]" + "\n")
		for _, child := range n.Children {
			w.printTree(child, depth+1)
		}
	case *SymbolNode:
		fmt.Printf("(S) " + w.Lexer.GetRuleNames()[n.StartState.GetRuleIndex()] + " [" + fmt.Sprint(n.StartState.GetRuleIndex()) + "]" + "\n")
		for _, child := range n.Children {
			w.printTree(child, depth+1)
		}
	case *LiteralNode:
		fmt.Printf("(L) '" + string(n.Text) + "'\n")
	default:
		panic(fmt.Sprintf("Tree contains other nodes than Rule-, Symbol-, or Literal-nodes (%T)\n", n))
	}
}

func (w *ATNWalker) exceededDeadline() bool {
	return w.deadlineIsSet && time.Now().After(w.deadline)
}

func (w *ATNWalker) AssembleTree(decoder *Decoder, root *RuleNode, stack *Stack[TreeNode]) bool {
	var node TreeNode
	var children []TreeNode
	stack = &Stack[TreeNode]{}
	stack.Push(root)
	for !stack.IsEmpty() {
		if w.exceededDeadline() {
			return false
		}
		node = stack.Pop()
		switch n := node.(type) {
		case *RuleNode:
			w.decodeParserRuleATN(decoder, n)
			children = n.Children
			for i := len(children) - 1; i >= 0; i-- {
				stack.Push(children[i])
			}
		case *SymbolNode:
			w.decodeLexerSymbolATN(decoder, n)
			children = n.GetChildren()
			for i := len(children) - 1; i >= 0; i-- {
				stack.Push(children[i])
			}
		}
	}
	return true
}

func (w *ATNWalker) TreeToString(root *RuleNode, stack *Stack[TreeNode]) string {
	// assemble the string with the parse tree (depth first)
	builder := strings.Builder{}
	var node TreeNode
	var children []TreeNode
	stack.Push(root)
	for !stack.IsEmpty() {
		if w.exceededDeadline() {
			return ""
		}
		node = stack.Pop()
		children = node.GetChildren()
		for i := len(children) - 1; i >= 0; i-- {
			stack.Push(children[i])
		}
		switch n := node.(type) {
		case *LiteralNode:
			builder.WriteRune(n.Text)
		}
	}
	return builder.String()
}

func (w *ATNWalker) Repair(data []byte) []byte {
	writeBack := &([]byte{})
	decoder := NewDecoder(data,
		len(w.Parser.GetATN().GetRuleIndexToStartStateSlice()),
		len(w.Lexer.GetATN().GetRuleIndexToStartStateSlice()), writeBack)
	stack := &Stack[TreeNode]{}
	root := NewRuleNode(nil, w.Parser.GetATN().GetRuleIndexToStartStateSlice()[0])
	if !w.AssembleTree(decoder, root, stack) {
		writeBack = nil
		return []byte{}
	}

	return decoder.writeBackEncoder.Bytes()
}

func (w *ATNWalker) Decode(data []byte, writeBack *[]byte) string {
	decoder := NewDecoder(data,
		len(w.Parser.GetATN().GetRuleIndexToStartStateSlice()),
		len(w.Lexer.GetATN().GetRuleIndexToStartStateSlice()), writeBack)
	stack := &Stack[TreeNode]{}
	root := NewRuleNode(nil, w.Parser.GetATN().GetRuleIndexToStartStateSlice()[0])
	if !w.AssembleTree(decoder, root, stack) {
		writeBack = nil
		return ""
	}

	if writeBack != nil {
		*writeBack = decoder.writeBackEncoder.Bytes()
	}

	return w.TreeToString(root, stack)
}
