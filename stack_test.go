package atnwalk

import "testing"

type stackProperties struct {
	length, capacity, top, bottom int
}

func (expected *stackProperties) assertValid(msg string, s *Stack[int], t *testing.T) {
	if expected.length != len(s.data) {
		t.Errorf("%v: expected len(s.data) == %v but got %v", msg, expected.length, len(s.data))
	}
	if expected.capacity != cap(s.data) {
		t.Errorf("%v: expected cap(s.data) == %v but got %v", msg, expected.capacity, cap(s.data))
	}
	ele := s.Top()
	if expected.top != ele {
		t.Errorf("%v: expected s.Top() == %v but got %v", msg, expected.top, ele)
	}
	ele = s.Bottom()
	if expected.bottom != ele {
		t.Errorf("%v: expected s.Bottom() == %v but got %v", msg, expected.bottom, ele)
	}
}

func TestStack(t *testing.T) {
	var properties *stackProperties
	s := &Stack[int]{}
	if !s.IsEmpty() {
		t.Errorf("exepected s.IsEmpty() == %v but got %v", true, s.IsEmpty())
	}

	s.Push(42)
	properties = &stackProperties{
		length:   1,
		capacity: 2,
		top:      42,
		bottom:   42}
	properties.assertValid("PUSH: 42", s, t)

	s.Push(999)
	properties = &stackProperties{
		length:   2,
		capacity: 2,
		top:      999,
		bottom:   42}
	properties.assertValid("PUSH: 999", s, t)

	s.Push(5, 6, 7, 8, 9)
	properties = &stackProperties{
		length:   7,
		capacity: 14,
		top:      9,
		bottom:   42}
	properties.assertValid("PUSH: 5, 6, 7, 8, 9", s, t)

	ele := s.Pop()
	if ele != 9 {
		t.Errorf("POP: 9: expected ele == %v but got %v", 9, ele)
	}
	properties = &stackProperties{
		length:   6,
		capacity: 14,
		top:      8,
		bottom:   42}
	properties.assertValid("POP: 9", s, t)

	expected := []int{8, 7, 6, 5, 999}
	for i := 0; i < len(expected); i++ {
		ele = s.Pop()
		if expected[i] != ele {
			t.Errorf("POP: 8, 7, 6, 5, 999: expected ele == %v but got %v", expected[i], ele)
		}
	}
	properties = &stackProperties{
		length:   1,
		capacity: 14,
		top:      42,
		bottom:   42}
	properties.assertValid("POP: 8, 7, 6, 5, 999", s, t)

	ele = s.Pop()
	if ele != 42 {
		t.Errorf("POP: 42: expected ele == %v but got %v", 42, ele)
	}

	if !s.IsEmpty() {
		t.Errorf("exepected s.IsEmpty() == %v but got %v", true, s.IsEmpty())
	}

	s.Push(21)
	properties = &stackProperties{
		length:   1,
		capacity: 14,
		top:      21,
		bottom:   21}
	properties.assertValid("PUSH: 21", s, t)

	bulkData := make([]int, 100)
	for i := 0; i < len(bulkData); i++ {
		bulkData[i] = i + 100
	}
	s.Push(bulkData...)

	properties = &stackProperties{
		length:   101,
		capacity: 202,
		top:      199,
		bottom:   21}
	properties.assertValid("PUSH: bulkData...", s, t)

	s.Push(bulkData...)
	properties = &stackProperties{
		length:   201,
		capacity: 202,
		top:      199,
		bottom:   21}
	properties.assertValid("PUSH: bulkData... (2nd run)", s, t)

	s.Push(456)
	properties = &stackProperties{
		length:   202,
		capacity: 202,
		top:      456,
		bottom:   21}
	properties.assertValid("PUSH: 456", s, t)

	s.Push(789)
	properties = &stackProperties{
		length:   203,
		capacity: 406,
		top:      789,
		bottom:   21}
	properties.assertValid("PUSH: 789", s, t)

	expected = []int{789, 456, 199, 198}
	for i := 0; i < len(expected); i++ {
		ele = s.Pop()
		if expected[i] != ele {
			t.Errorf("POP: 789, 456, 199, 198: expected ele == %v but got %v", expected[i], ele)
		}
	}
	properties = &stackProperties{
		length:   199,
		capacity: 406,
		top:      197,
		bottom:   21}
	properties.assertValid("PUSH: 789, 456, 199, 198", s, t)
}
