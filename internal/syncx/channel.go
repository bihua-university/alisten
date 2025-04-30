package syncx

type UnboundedChan[T any] struct {
	in  chan<- T
	out <-chan T
}

func (c *UnboundedChan[T]) In() chan<- T {
	return c.in
}

func (c *UnboundedChan[T]) Out() <-chan T {
	return c.out
}

func NewUnboundedChan[T any](capacity int) UnboundedChan[T] {
	in := make(chan T, capacity)
	out := make(chan T, capacity)

	// spawn background loop
	go func() {
		defer close(out)
		buffer := make([]T, 0, capacity)

	forward:
		for {
			val, ok := <-in
			if !ok {
				break forward
			}

			// try forward to out
			select {
			case out <- val:
				continue
			default:
			}

			// out is full, put val in buffer
			buffer = append(buffer, val)
			for len(buffer) > 0 {
				select {
				case val, ok := <-in:
					if !ok {
						break forward
					}
					buffer = append(buffer, val)
				case out <- buffer[0]:
					buffer = buffer[1:]
					if len(buffer) == 0 {
						// make a new buffer to avoid memory leak
						buffer = make([]T, 0, capacity)
					}
				}
			}
		}

		// drain the remain values
		for len(buffer) > 0 {
			out <- buffer[0]
			buffer = buffer[1:]
		}
	}()

	return UnboundedChan[T]{in: in, out: out}
}
