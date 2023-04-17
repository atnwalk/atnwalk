package antlr

func (s *IntervalSet) Contains(i int) bool {
	return s.contains(i)
}

// func (s *IntervalSet) GetIntervals() []*Interval {
// 	return s.intervals
// }

func (s *IntervalSet) Length() int {
	return s.length()
}

func (s *IntervalSet) Complement() *IntervalSet {
	return s.complement(0, LexerMaxCharValue)
}

func (s *IntervalSet) GetIndex(i int) int {
	count := 0
	for _, interval := range s.intervals {
		if i >= interval.Start && i < interval.Stop {
			return i - interval.Start + count
		}
		count += interval.length()
	}
	panic("Provided item not in IntervalSet")
}

func (s *IntervalSet) Get(index int) int {
	count := 0
	for _, interval := range s.intervals {
		if index < count+interval.length() {
			return interval.Start + index - count
		}
		count += interval.length()
	}
	panic("Provided index is out of range")
}

type AnyTransition interface {
	GetTarget() ATNState
}

func (t *BaseTransition) GetLabelValue() int {
	return t.label
}

func (t *BaseTransition) GetLabel() *IntervalSet {
	return t.getLabel()
}

func (t *BaseTransition) GetTarget() ATNState {
	return t.getTarget()
}

func (t *RuleTransition) GetRuleIndex() int {
	return t.ruleIndex
}

func (t *RuleTransition) GetFollowState() ATNState {
	return t.followState
}

func (atn *ATN) GetStates() []ATNState {
	return atn.states
}

func (atn *ATN) GetRuleIndexToStartStateSlice() []*RuleStartState {
	return atn.ruleToStartState
}

func (atn *ATN) GetRuleIndexToStopStateSlice() []*RuleStopState {
	return atn.ruleToStopState
}
