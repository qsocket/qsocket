package qsnetcat

import (
	"bytes"
	"encoding/gob"

	"github.com/qsocket/qs-netcat/config"
	"github.com/qsocket/qsocket-go"
)

type SessionSpecs struct {
	Command     string
	ForwardAddr string
	TermSize    Winsize
	Interactive bool
}

// Winsize describes the terminal window size
type Winsize struct {
	Rows uint16 // ws_row: Number of rows (in cells)
	Cols uint16 // ws_col: Number of columns (in cells)
	X    uint16 // ws_xpixel: Width in pixels
	Y    uint16 // ws_ypixel: Height in pixels
}

func SendSessionSpecs(qs *qsocket.QSocket, opts *config.Options) error {
	if qs.IsClosed() {
		return qsocket.ErrSocketNotConnected
	}

	ws := new(Winsize)
	err := error(nil)
	if !opts.IsPiped() {
		ws, err = GetCurrentTermSize()
		if err != nil {
			return err
		}
	}

	specs := SessionSpecs{
		Command:     opts.Execute,
		ForwardAddr: opts.ForwardAddr,
		TermSize: Winsize{
			Cols: ws.Cols,
			Rows: ws.Rows,
			X:    ws.X,
			Y:    ws.Y,
		},
		Interactive: opts.Interactive,
	}
	buf := bytes.Buffer{}
	enc := gob.NewEncoder(&buf)

	if err := enc.Encode(specs); err != nil {
		return err
	}

	_, err = qs.Write(buf.Bytes())
	return err
}

func RecvSessionSpecs(qs *qsocket.QSocket, opts *config.Options) (*SessionSpecs, error) {
	if qs.IsClosed() {
		return nil, qsocket.ErrSocketNotConnected
	}
	specs := new(SessionSpecs)
	data := make([]byte, 512)
	n, err := qs.Read(data)
	if err != nil {
		return nil, err
	}
	buf := bytes.NewBuffer(data[:n])
	dec := gob.NewDecoder(buf)

	if err := dec.Decode(specs); err != nil {
		return nil, err
	}

	if specs.TermSize.Cols == 0 &&
		specs.TermSize.Rows == 0 &&
		!opts.IsPiped() {
		ws, err := GetCurrentTermSize()
		if err != nil {
			return nil, err
		}
		specs.TermSize = Winsize{
			Cols: ws.Cols,
			Rows: ws.Rows,
		}
	}

	if specs.Command == "" {
		specs.Command = opts.Execute
	}

	return specs, nil
}
