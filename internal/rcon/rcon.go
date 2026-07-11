package rcon

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"time"
)

const (
	packetAuth    = 3
	packetCommand = 2
)

type Client struct {
	Addr     string
	Password string
	Timeout  time.Duration
}

func (c Client) Command(command string) (string, error) {
	timeout := c.Timeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	conn, err := net.DialTimeout("tcp", c.Addr, timeout)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(timeout))

	if err := writePacket(conn, 1, packetAuth, c.Password); err != nil {
		return "", err
	}
	id, _, _, err := readPacket(conn)
	if err != nil {
		return "", err
	}
	if id == -1 {
		return "", fmt.Errorf("RCON authentication failed")
	}
	if err := writePacket(conn, 2, packetCommand, command); err != nil {
		return "", err
	}
	_, _, body, err := readPacket(conn)
	return body, err
}

func writePacket(w io.Writer, id, typ int32, body string) error {
	payload := new(bytes.Buffer)
	_ = binary.Write(payload, binary.LittleEndian, id)
	_ = binary.Write(payload, binary.LittleEndian, typ)
	payload.WriteString(body)
	payload.WriteByte(0)
	payload.WriteByte(0)
	if err := binary.Write(w, binary.LittleEndian, int32(payload.Len())); err != nil {
		return err
	}
	_, err := w.Write(payload.Bytes())
	return err
}

func readPacket(r io.Reader) (int32, int32, string, error) {
	var size int32
	if err := binary.Read(r, binary.LittleEndian, &size); err != nil {
		return 0, 0, "", err
	}
	if size < 10 || size > 1024*1024 {
		return 0, 0, "", fmt.Errorf("invalid RCON packet size %d", size)
	}
	buf := make([]byte, size)
	if _, err := io.ReadFull(r, buf); err != nil {
		return 0, 0, "", err
	}
	id := int32(binary.LittleEndian.Uint32(buf[0:4]))
	typ := int32(binary.LittleEndian.Uint32(buf[4:8]))
	body := string(bytes.TrimRight(buf[8:], "\x00"))
	return id, typ, body, nil
}
