package store

import (
	"github.com/cbodonnell/chrono/pkg/entity"
	"github.com/vmihailenco/msgpack/v5"
)

type MsgpackSerializer struct{}

func NewMsgpackSerializer() *MsgpackSerializer {
	return &MsgpackSerializer{}
}

func (s *MsgpackSerializer) Marshal(e *entity.Entity) ([]byte, error) {
	return msgpack.Marshal(e)
}

func (s *MsgpackSerializer) Unmarshal(data []byte, e *entity.Entity) error {
	return msgpack.Unmarshal(data, e)
}
