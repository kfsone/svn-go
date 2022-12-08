package main

// Helper provides a simple encapsulation of a goroutine that repeatedly applies
// work to a work queue until the queue closes.
type Helper[Work any, Ctx any] struct {
	work chan<- Work
	done <-chan bool
}

// NewHelper creates a worker consuming work items from a channel and applying
// the given function to each item, until the work channel is closed, with a
// means of waiting until all work has been consumed. The size of the work
// channel is specified via 'workCap'.
func NewHelper[Work any, Ctx any](workCap int, fn func(Work, Ctx), ctx Ctx) *Helper[Work, Ctx] {
	work := make(chan Work, workCap)
	done := make(chan bool, 1)

	go func() {
		done, work := done, work
		defer close(done)
		for item := range work {
			fn(item, ctx)
		}
		done <- true
	}()

	return &Helper[Work, Ctx]{work: work, done: done}
}

// Close signals the helper that the last work has been dispatched, and it should
// exit once the queue is drained.
func (h *Helper[Work, Ctx]) Close() {
	close(h.work)
}

// Wait blocks until the helper has finished processing all work and consumed
// the Close signal.
func (h *Helper[Work, Ctx]) Wait() {
	<-h.done
}

// Queue sends an item of work to the helper.
func (h *Helper[Work, Ctx]) Queue(item Work) {
	h.work <- item
}

// CloseWait is a convenience functinon that calls Close and then Wait.
func (h *Helper[Work, Ctx]) CloseWait() {
	h.Close()
	h.Wait()
}
