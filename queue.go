package atnwalk

type Queue[T any] struct {
	data []T
	zero T
	head int
	tail int
	size int
}

func (q *Queue[T]) Enqueue(item ...T) {
	if len(item) == 0 {
		panic("no item")
	}
	freeSpace := len(q.data) - q.size
	if freeSpace < len(item) {
		newCapacity := (q.size + len(item)) << 1
		newBuffer := make([]T, newCapacity)
		if q.head < q.tail {
			copy(newBuffer, q.data[q.head:q.tail])
			q.tail = q.tail - q.head
		} else {
			copy(newBuffer, q.data[q.head:])
			copy(newBuffer[len(q.data)-q.head:], q.data[:q.tail])
			q.tail = len(q.data) - q.head + q.tail
		}
		q.head = 0
		q.data = newBuffer
	}
	for i, v := range item {
		q.data[(q.tail+i)%len(q.data)] = v
	}
	q.tail = (q.tail + len(item)) % len(q.data)
	q.size += len(item)
}

func (q *Queue[T]) Dequeue() T {
	if q.size == 0 {
		panic("empty queue")
	}
	value := q.data[q.head]
	q.data[q.head] = q.zero
	q.head = (q.head + 1) % len(q.data)
	q.size--
	return value
}

func (q *Queue[T]) Front() T {
	if q.size == 0 {
		panic("empty queue")
	}
	return q.data[q.head]
}

func (q *Queue[T]) Rear() T {
	if q.size == 0 {
		panic("empty queue")
	}
	if q.tail == 0 {
		return q.data[len(q.data)-1]
	}
	return q.data[q.tail-1]
}

func (q *Queue[T]) Size() int {
	return q.size
}

func (q *Queue[T]) IsEmpty() bool {
	return q.size == 0
}
