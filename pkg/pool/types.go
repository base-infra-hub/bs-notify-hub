package pool

// Connection 连接池的连接对象定义
type Connection interface {
	Write(data interface{}) error
	Close() error
}
