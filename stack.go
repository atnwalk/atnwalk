package atnwalk

type Stack[T any] struct {
	data []T
	zero T
}

func (s *Stack[T]) Push(item ...T) {
	if len(item) == 0 {
		panic("no item")
	}
	if (cap(s.data) - len(s.data)) < len(item) {
		newBuffer := make([]T, 0, (len(s.data)+len(item))<<1)
		s.data = append(newBuffer, s.data...)
	}
	s.data = append(s.data, item...)
}

func (s *Stack[T]) Pop() T {
	if len(s.data) == 0 {
		panic("empty stack")
	}
	value := s.data[len(s.data)-1]
	s.data[len(s.data)-1] = s.zero
	s.data = s.data[:len(s.data)-1]
	return value
}

func (s *Stack[T]) Top() T {
	if len(s.data) == 0 {
		panic("empty stack")
	}
	return s.data[len(s.data)-1]
}

func (s *Stack[T]) Bottom() T {
	if len(s.data) == 0 {
		panic("empty stack")
	}
	return s.data[0]
}

func (s *Stack[T]) Size() int {
	return len(s.data)
}

func (s *Stack[T]) IsEmpty() bool {
	return len(s.data) == 0
}
