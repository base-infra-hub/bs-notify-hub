package dispatch

import "fmt"

type LocalBroker struct {
	ch chan *Message
}

func NewLocalBroker(buffer int) *LocalBroker {
	return &LocalBroker{
		ch: make(chan *Message, buffer),
	}
}

func (b *LocalBroker) Publish(msg *Message) error {
	select {
	case b.ch <- msg:
		return nil
	default:
		return fmt.Errorf("本地队列已满")
	}
}

func (b *LocalBroker) Subscribe() <-chan *Message {
	return b.ch
}
