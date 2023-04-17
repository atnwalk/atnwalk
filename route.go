package atnwalk

import (
	"math/rand"
	"sync"

	"github.com/antlr/antlr4/runtime/Go/antlr/v4"
)

const (
	Zero = iota
	NonRecursive
	Recursive
)

type Router struct {
	mutex          sync.Mutex
	stateToOptions map[int]*RouteOptions
	decoder        *Decoder
	atn            *antlr.ATN
	startState     int
	stopState      int
	ruleIndex      int
	nextChoices    *Stack[int]
}

func NewRouter(ruleIndex, startState, stopState int, atn *antlr.ATN, decoder *Decoder) *Router {
	return &Router{
		stateToOptions: map[int]*RouteOptions{},
		decoder:        decoder,
		atn:            atn,
		startState:     startState,
		stopState:      stopState,
		ruleIndex:      ruleIndex,
		nextChoices:    &Stack[int]{}}
}

type RouteOptions struct {
	choiceToNextState               []int
	notVisitedChoices               []int
	bucketToChoices                 [3][]int
	nonRecursiveChoiceToRuleIndices map[int]map[int]struct{}
}

func (r *Router) NewRouteOptions(state int) *RouteOptions {
	numChoices := len(r.atn.GetStates()[state].GetTransitions())
	choiceToNextState := make([]int, numChoices)
	notVisitedChoices := make([]int, numChoices)
	for i := 0; i < numChoices; i++ {
		choiceToNextState[i] = -128
		notVisitedChoices[i] = i
	}
	return &RouteOptions{
		choiceToNextState:               choiceToNextState,
		notVisitedChoices:               notVisitedChoices,
		bucketToChoices:                 [3][]int{},
		nonRecursiveChoiceToRuleIndices: make(map[int]map[int]struct{}, numChoices)}
}

type RouteEdge struct {
	src    int
	dest   int
	choice int
	// the rule indices observed when using the choice from the last state to the current state
	rules []int
}

func (r *Router) LearnRoutes(queue <-chan *RouteEdge, ok chan<- bool) {
	for item := range queue {

		// if the received RouteEdge is nil then send to the "ok" channel
		// that the sequence of edges has been processed so far
		// i.e., providing nil is a control sequence to respond to
		if item == nil {
			ok <- true
			continue
		}

		// get the route options or create them if they do not exist
		routeOptions, ok := r.stateToOptions[item.src]
		if !ok {
			routeOptions = r.NewRouteOptions(item.src)
			r.stateToOptions[item.src] = routeOptions
		}

		// check whether this choice is not visited
		// this seems like a linear inefficiency, however, the number of choices is mostly small
		// the order is somewhat important for deterministic routing later
		// and it seems that using a map[int]struct{} is probably not faster
		// and less memory efficient... though have to confirm that later by benchmarking it
		// TODO
		foundAt := -1
		for i := 0; i < len(routeOptions.notVisitedChoices); i++ {
			if routeOptions.notVisitedChoices[i] == item.choice {
				foundAt = i
				break
			}
		}

		// continue if this choice of the state was already visited
		// found in "not visited choices" --> not visited, otherwise visited
		if foundAt < 0 {
			continue
		}

		// remove the choice from the "not visited" set
		routeOptions.notVisitedChoices[foundAt] = routeOptions.notVisitedChoices[len(routeOptions.notVisitedChoices)-1]
		routeOptions.notVisitedChoices = routeOptions.notVisitedChoices[:len(routeOptions.notVisitedChoices)-1]

		// determine the bucket
		bucket := Zero
		if len(item.rules) > 0 {
			bucket = NonRecursive
			for i := 0; i < len(item.rules); i++ {
				if item.rules[i] == r.ruleIndex {
					bucket = Recursive
					break
				}
			}
		}

		// put the choice into a bucket
		routeOptions.bucketToChoices[bucket] = append(routeOptions.bucketToChoices[bucket], item.choice)
		routeOptions.choiceToNextState[item.choice] = item.dest

		// track which rules were observed if the choice is a non-recursive rule choice
		if bucket == NonRecursive {
			choiceToRules := make(map[int]struct{}, len(item.rules))
			for _, ruleIndex := range item.rules {
				choiceToRules[ruleIndex] = struct{}{}
			}
			routeOptions.nonRecursiveChoiceToRuleIndices[item.choice] = choiceToRules
		}

		// TODO: maybe implement the below statements at some point
		// if v.dest == r.stopState {
		// 	break
		// }
	}
}

type RouteNode struct {
	depth        int
	state        int
	prevChoice   int
	prevNode     *RouteNode
	routeOptions *RouteOptions
}

func (r *Router) NewRouteNode(state, prevChoice, depth int, prevNode *RouteNode) *RouteNode {
	routeOptions := r.stateToOptions[state]
	return &RouteNode{
		depth:        depth,
		state:        state,
		prevChoice:   prevChoice,
		prevNode:     prevNode,
		routeOptions: routeOptions}
}

type PriorityQueue struct {
	Router                   *Router
	ZeroNodes                Stack[*RouteNode]
	TransitiveRecursiveNodes Queue[*RouteNode]
	SelfRecursiveNodes       Queue[*RouteNode]
	VisitedStates            map[int]struct{}
	BestNotVisitedNode       *RouteNode
	PRNGSource               rand.Source
}

func (p *PriorityQueue) Evaluate(node *RouteNode, rootPathRules map[int]struct{}) *RouteNode {

	// by evaluating the current node, update the best node that has unvisited choices if necessary
	if len(node.routeOptions.notVisitedChoices) > 0 {
		if p.BestNotVisitedNode == nil || node.depth < p.BestNotVisitedNode.depth {
			p.BestNotVisitedNode = node
		}
	}

	seed := int(p.PRNGSource.Int63()) //p.Router.decoder.Decode(math.MaxInt32)
	for bucket := Zero; bucket <= Recursive; bucket++ {
		n := len(node.routeOptions.bucketToChoices[bucket])
		for i := seed; i < seed+n; i++ {
			nextChoice := node.routeOptions.bucketToChoices[bucket][i%n]
			nextState := node.routeOptions.choiceToNextState[nextChoice]
			if _, ok := p.VisitedStates[nextState]; !ok {
				nextNode := p.Router.NewRouteNode(nextState, nextChoice, node.depth+1, node)
				p.VisitedStates[nextState] = struct{}{}
				switch bucket {
				case Zero:
					p.ZeroNodes.Push(nextNode)
				case NonRecursive:
					isTransitiveRecursive := false
					for ruleIndex := range rootPathRules {
						if _, ok := node.routeOptions.nonRecursiveChoiceToRuleIndices[nextChoice][ruleIndex]; ok {
							isTransitiveRecursive = true
							break
						}
					}
					if isTransitiveRecursive {
						p.TransitiveRecursiveNodes.Enqueue(nextNode)
					} else {
						p.ZeroNodes.Push(nextNode)
					}
				case Recursive:
					p.SelfRecursiveNodes.Enqueue(nextNode)
				}
			}
		}
	}

	switch {
	case !p.ZeroNodes.IsEmpty():
		return p.ZeroNodes.Pop()
	case p.BestNotVisitedNode != nil:
		return p.BestNotVisitedNode
	case !p.TransitiveRecursiveNodes.IsEmpty():
		return p.TransitiveRecursiveNodes.Dequeue()
	case !p.SelfRecursiveNodes.IsEmpty():
		return p.SelfRecursiveNodes.Dequeue()
	}
	return nil
}

// NotVisitedNodeIfViable returns the best node that contains not yet visited transitions if no zero paths were found
// so far, otherwise nil.
func (p *PriorityQueue) NotVisitedNodeIfViable() *RouteNode {
	if p.ZeroNodes.IsEmpty() {
		return p.BestNotVisitedNode
	}
	return nil
}

func NewPriorityQueue(router *Router, initialState int, prngSource rand.Source) *PriorityQueue {
	return &PriorityQueue{
		Router:                   router,
		ZeroNodes:                Stack[*RouteNode]{},
		TransitiveRecursiveNodes: Queue[*RouteNode]{},
		SelfRecursiveNodes:       Queue[*RouteNode]{},
		VisitedStates:            map[int]struct{}{initialState: {}},
		BestNotVisitedNode:       nil,
		PRNGSource:               prngSource}
}

func (r *Router) route(state int, rootPathRules map[int]struct{}) int {
	// route with the previously found choices
	if !r.nextChoices.IsEmpty() {
		return r.nextChoices.Pop()
	}

	// just return a random choice, nothing about this state has been previously learned
	if _, ok := r.stateToOptions[state]; !ok {
		return int(r.decoder.prngSource.Int63()) % len(r.atn.GetStates()[state].GetTransitions())
		// return r.decoder.Decode(len(r.atn.GetStates()[state].GetTransitions()))
	}

	// Dijkstra algorithm with priority queue, priority:
	// 1. Zero paths (depth first, epsilon, token, or rule transitions that are not recursive)
	// 2. Unknown paths (breadth first, last node in path has unknown transitions to follow, prefer the shortest depth)
	// 3. Transitive recursive paths (contain at least one rule that is a parent further up the parse tree)
	// 4. Recursive paths (contain at least one rule that is the same which is currently decoded)
	priorityQueue := NewPriorityQueue(r, state, r.decoder.prngSource)
	node := r.NewRouteNode(state, -127, 0, nil)
	for node == nil || node.state != r.stopState {
		if notVisitedNode := priorityQueue.NotVisitedNodeIfViable(); notVisitedNode != nil {
			node = notVisitedNode
			break
		}
		node = priorityQueue.Evaluate(node, rootPathRules)
	}

	if node.state != r.stopState {
		// r.nextChoices.Push(node.routeOptions.notVisitedChoices[r.decoder.Decode(len(node.routeOptions.notVisitedChoices))])
		r.nextChoices.Push(node.routeOptions.notVisitedChoices[int(r.decoder.prngSource.Int63())%len(node.routeOptions.notVisitedChoices)])
	}

	for node.prevNode != nil {
		r.nextChoices.Push(node.prevChoice)
		node = node.prevNode
	}

	return r.nextChoices.Pop()
}
