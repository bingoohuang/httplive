package lb

import (
	"net"
	"time"

	"github.com/valyala/fasthttp"
)

// https://gist.github.com/c4milo/275abc6eccbfd88ad56ca7c77947883a

// TimeoutDialContext implements our own dialer in order to set read and write idle timeouts.
func TimeoutDialContext(rwtimeout, ctimeout time.Duration) fasthttp.DialFunc {
	return func(addr string) (net.Conn, error) {
		c, err := net.DialTimeout("tcp", addr, ctimeout)
		if err != nil {
			return nil, err
		}

		if rwtimeout > 0 {
			c = &tcpConn{Conn: c, timeout: rwtimeout}
		}

		return c, nil
	}
}

// tcpConn is our own net.Conn which sets a read and write deadline and resets them each
// time there is read or write activity in the connection.
type tcpConn struct {
	net.Conn
	timeout time.Duration
}

// Read implements the Conn Read method.
func (c *tcpConn) Read(b []byte) (int, error) {
	err := c.Conn.SetDeadline(time.Now().Add(c.timeout))
	if err != nil {
		return 0, err
	}

	return c.Conn.Read(b)
}

// Write implements the Conn Write method.
func (c *tcpConn) Write(b []byte) (int, error) {
	err := c.Conn.SetDeadline(time.Now().Add(c.timeout))
	if err != nil {
		return 0, err
	}

	return c.Conn.Write(b)
}
