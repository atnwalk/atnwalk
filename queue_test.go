package atnwalk

import "testing"

type queueProperties struct {
	size, capacity, front, rear, head, tail int
}

func (expected *queueProperties) assertValid(msg string, q *Queue[int], t *testing.T) {

	if q.size != expected.size {
		t.Errorf("%v: expected q.size == %v but got %v", msg, expected.size, q.size)
	}

	if cap(q.data) != expected.capacity {
		t.Errorf("%v: expected cap(q.data) == %v but got %v", msg, expected.capacity, cap(q.data))
	}

	if q.head != expected.head {
		t.Errorf("%v: expected q.head == %v but got %v", msg, expected.head, q.head)
	}

	if q.tail != expected.tail {
		t.Errorf("%v: expected q.tail == %v but got %v", msg, expected.tail, q.tail)
	}

	var val int
	val = q.Front()
	if val != expected.front {
		t.Errorf("%v: expected q.Front() == %v but got %v", msg, expected.front, val)
	}

	val = q.Rear()
	if val != expected.rear {
		t.Errorf("%v: expected q.Rear() == %v but got %v", msg, expected.rear, val)
	}
}

func TestQueue(t *testing.T) {

	var properties *queueProperties
	q := &Queue[int]{}
	if !q.IsEmpty() {
		t.Errorf("exepected q.IsEmpty() == %v but got %v", true, q.IsEmpty())
	}

	// ENQUEUE: 1 (size: 0, capacity: 0) --> (size: 1, capacity: 2)
	q.Enqueue(42)
	properties = &queueProperties{
		size:     1,
		capacity: 2,
		head:     0,
		tail:     1,
		front:    42,
		rear:     42}
	properties.assertValid("ENQUEUE: 1", q, t)

	// ENQUEUE: 999 (size: 1, capacity: 2) --> (size: 2, capacity: 2)
	// should NOT increase the capacity
	q.Enqueue(999)
	properties = &queueProperties{
		size:     2,
		capacity: 2,
		head:     0,
		tail:     0,
		front:    42,
		rear:     999}
	properties.assertValid("ENQUEUE: 999", q, t)

	// ENQUEUE: 6, 7, 8, 9 (size: 2, capacity: 2) --> (size: 6, capacity: 12)
	q.Enqueue(6, 7, 8, 9)
	properties = &queueProperties{
		size:     6,
		capacity: 12,
		head:     0,
		tail:     6,
		front:    42,
		rear:     9}
	properties.assertValid("ENQUEUE: 6, 7, 8, 9", q, t)

	// DEQUEUE: 42 (size: 6, capacity: 12) --> (size: 5, capacity: 12)
	ele := q.Dequeue()
	if ele != 42 {
		t.Errorf("DEQUEUE: 42: expected ele == %v but got %v", 42, ele)
	}
	properties = &queueProperties{
		size:     5,
		capacity: 12,
		head:     1,
		tail:     6,
		front:    999,
		rear:     9}
	properties.assertValid("DEQUEUE: 42", q, t)

	// DEQUEUE: 999 (size: 5, capacity: 12) --> (size: 4, capacity: 12)
	ele = q.Dequeue()
	if ele != 999 {
		t.Errorf("DEQUEUE: 999: expected ele == %v but got %v", 999, ele)
	}
	properties = &queueProperties{
		size:     4,
		capacity: 12,
		head:     2,
		tail:     6,
		front:    6,
		rear:     9}
	properties.assertValid("DEQUEUE: 999", q, t)

	// DEQUEUE (size: 4, capacity: 12) --> (size: 3, capacity: 12)
	ele = q.Dequeue()
	if ele != 6 {
		t.Errorf("DEQUEUE: 6: expected ele == %v but got %v", 6, ele)
	}
	properties = &queueProperties{
		size:     3,
		capacity: 12,
		head:     3,
		tail:     6,
		front:    7,
		rear:     9}
	properties.assertValid("DEQUEUE: 6", q, t)

	// ENQUEUE: 21, 22, 23, 24, 25 (size: 3, capacity: 12) --> (size: 8, capacity: 12)
	q.Enqueue(21, 22, 23, 24, 25)
	properties = &queueProperties{
		size:     8,
		capacity: 12,
		head:     3,
		tail:     11,
		front:    7,
		rear:     25}
	properties.assertValid("ENQUEUE: 21, 22, 23, 24, 25", q, t)

	// ENQUEUE: 26 (size: 8, capacity: 12) --> (size: 9, capacity: 12)
	q.Enqueue(26)
	properties = &queueProperties{
		size:     9,
		capacity: 12,
		head:     3,
		tail:     0,
		front:    7,
		rear:     26}
	properties.assertValid("ENQUEUE: 26", q, t)

	// ENQUEUE: 31, 32, 33 (size: 8, capacity: 12) --> (size: 12, capacity: 12)
	// should start to recycle first slots of the data slice
	q.Enqueue(31, 32, 33)
	properties = &queueProperties{
		size:     12,
		capacity: 12,
		head:     3,
		tail:     3,
		front:    7,
		rear:     33}
	properties.assertValid("ENQUEUE: 31, 32, 33", q, t)

	// should return the next six elements in order
	expected := []int{7, 8, 9, 21, 22, 23}
	for i := 0; i < 6; i++ {
		ele = q.Dequeue()
		if expected[i] != ele {
			t.Errorf("DEQUEUE: 7, 8, 9, 21, 22, 23: expected ele == %v but got %v", expected[i], ele)
		}
	}
	properties = &queueProperties{
		size:     6,
		capacity: 12,
		head:     9,
		tail:     3,
		front:    24,
		rear:     33}
	properties.assertValid("DEQUEUE: 7, 8, 9, 21, 22, 23", q, t)

	// should return the next six elements in order
	expected = []int{24, 25, 26, 31, 32, 33}
	for i := 0; i < len(expected); i++ {
		ele = q.Dequeue()
		if expected[i] != ele {
			t.Errorf("DEQUEUE: 24, 25, 26, 31, 32, 33: expected ele == %v but got %v", expected[i], ele)
		}
	}
	if !q.IsEmpty() {
		t.Errorf("Exepected q.IsEmpty() == %v but got %v", true, q.IsEmpty())
	}

	// fill the queue until its max capacity
	q.Enqueue(1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12)
	properties = &queueProperties{
		size:     12,
		capacity: 12,
		head:     3,
		tail:     3,
		front:    1,
		rear:     12}
	properties.assertValid("ENQUEUE: 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12", q, t)

	// let it resize
	// ENQUEUE: 34 (size: 12, capacity: 12) --> (size: 13, capacity: 26)
	q.Enqueue(34)
	properties = &queueProperties{
		size:     13,
		capacity: 26,
		head:     0,
		tail:     13,
		front:    1,
		rear:     34}
	properties.assertValid("ENQUEUE: 34", q, t)

	// DEQUEUE: 1 (size: 13, capacity: 26) --> (size: 12, capacity: 26)
	ele = q.Dequeue()
	if ele != 1 {
		t.Errorf("DEQUEUE: 1: expected ele == %v but got %v", 1, ele)
	}
	properties = &queueProperties{
		size:     12,
		capacity: 26,
		head:     1,
		tail:     13,
		front:    2,
		rear:     34}
	properties.assertValid("DEQUEUE: 1", q, t)

	// DEQUEUE: 2 (size: 12, capacity: 26) --> (size: 11, capacity: 26)
	ele = q.Dequeue()
	if ele != 2 {
		t.Errorf("DEQUEUE 2: expected ele == %v but got %v", 2, ele)
	}
	properties = &queueProperties{
		size:     11,
		capacity: 26,
		head:     2,
		tail:     13,
		front:    3,
		rear:     34}
	properties.assertValid("DEQUEUE 2", q, t)

	// ENQUEUE: bulkData... (size: 11, capacity: 26) --> (size: 111, capacity: 222)
	bulkData := make([]int, 100)
	for i := 0; i < len(bulkData); i++ {
		bulkData[i] = i + 100
	}
	q.Enqueue(bulkData...)
	properties = &queueProperties{
		size:     111,
		capacity: 222,
		head:     0,
		tail:     111,
		front:    3,
		rear:     199}
	properties.assertValid("ENQUEUE: bulkdata...", q, t)

	// DEQUEUE: 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 100, 101, 102, 103, 104  (size: 111, capacity: 222) --> (size: 96, capacity: 222)
	expected = []int{3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 34, 100, 101, 102, 103}
	for i := 0; i < len(expected); i++ {
		ele = q.Dequeue()
		if expected[i] != ele {
			t.Errorf("DEQUEUE: 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 34, 100, 101, 102, 103: expected ele == %v but got %v", expected[i], ele)
		}
	}
	properties = &queueProperties{
		size:     96,
		capacity: 222,
		head:     15,
		tail:     111,
		front:    104,
		rear:     199}
	properties.assertValid("DEQUEUE: 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 34, 100, 101, 102, 103", q, t)
}
