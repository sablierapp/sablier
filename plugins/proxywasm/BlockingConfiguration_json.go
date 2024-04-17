package main

import jsoniter "github.com/json-iterator/tinygo"

type BlockingConfiguration_json struct {
}
func (json BlockingConfiguration_json) Type() interface{} {
  var val BlockingConfiguration
  return val
}
func (json BlockingConfiguration_json) Unmarshal(iter *jsoniter.Iterator, out interface{}) {
  BlockingConfiguration_json_unmarshal(iter, out.(*BlockingConfiguration))
}
func (json BlockingConfiguration_json) Marshal(stream *jsoniter.Stream, val interface{}) {
  BlockingConfiguration_json_marshal(stream, val.(BlockingConfiguration))
}
func BlockingConfiguration_json_unmarshal(iter *jsoniter.Iterator, out *BlockingConfiguration) {
  more := iter.ReadObjectHead()
  for more {
    field := iter.ReadObjectField()
    if !BlockingConfiguration_json_unmarshal_field(iter, field, out) {
      iter.Skip()
    }
    more = iter.ReadObjectMore()
  }
}
func BlockingConfiguration_json_unmarshal_field(iter *jsoniter.Iterator, field string, out *BlockingConfiguration) bool {
  switch {
  case field == `timeout`:
    iter.ReadString(&(*out).Timeout)
    return true
  }
  return false
}
func BlockingConfiguration_json_marshal(stream *jsoniter.Stream, val BlockingConfiguration) {
    stream.WriteObjectHead()
    BlockingConfiguration_json_marshal_field(stream, val)
    stream.WriteObjectTail()
}
func BlockingConfiguration_json_marshal_field(stream *jsoniter.Stream, val BlockingConfiguration) {
    stream.WriteObjectField(`timeout`)
    stream.WriteString(val.Timeout)
    stream.WriteMore()
}
