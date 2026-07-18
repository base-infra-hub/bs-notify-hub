package dispatch

type Broker interface {
	Publish(msg *Message) error
	Subscribe() <-chan *Message
}
