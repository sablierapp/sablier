package main

import jsoniter "github.com/json-iterator/tinygo"

type DynamicConfiguration_json struct {
}
func (json DynamicConfiguration_json) Type() interface{} {
  var val DynamicConfiguration
  return val
}
func (json DynamicConfiguration_json) Unmarshal(iter *jsoniter.Iterator, out interface{}) {
  DynamicConfiguration_json_unmarshal(iter, out.(*DynamicConfiguration))
}
func (json DynamicConfiguration_json) Marshal(stream *jsoniter.Stream, val interface{}) {
  DynamicConfiguration_json_marshal(stream, val.(DynamicConfiguration))
}
func DynamicConfiguration_json_unmarshal(iter *jsoniter.Iterator, out *DynamicConfiguration) {
  more := iter.ReadObjectHead()
  for more {
    field := iter.ReadObjectField()
    if !DynamicConfiguration_json_unmarshal_field(iter, field, out) {
      iter.Skip()
    }
    more = iter.ReadObjectMore()
  }
}
func DynamicConfiguration_json_unmarshal_field(iter *jsoniter.Iterator, field string, out *DynamicConfiguration) bool {
  switch {
  case field == `display_name`:
    iter.ReadString(&(*out).DisplayName)
    return true
  case field == `show_details`:
    DynamicConfiguration_ptr1_json_unmarshal(iter, &(*out).ShowDetails)
    return true
  case field == `theme`:
    iter.ReadString(&(*out).Theme)
    return true
  case field == `refresh_frequency`:
    iter.ReadString(&(*out).RefreshFrequency)
    return true
  }
  return false
}
func DynamicConfiguration_ptr1_json_unmarshal (iter *jsoniter.Iterator, out **bool) {
    var val bool
    iter.ReadBool(&val)
    if iter.Error == nil {
      *out = &val
    }
}
func DynamicConfiguration_json_marshal(stream *jsoniter.Stream, val DynamicConfiguration) {
    stream.WriteObjectHead()
    DynamicConfiguration_json_marshal_field(stream, val)
    stream.WriteObjectTail()
}
func DynamicConfiguration_json_marshal_field(stream *jsoniter.Stream, val DynamicConfiguration) {
    stream.WriteObjectField(`display_name`)
    stream.WriteString(val.DisplayName)
    stream.WriteMore()
    stream.WriteObjectField(`show_details`)
    if val.ShowDetails == nil {
       stream.WriteNull()
    } else {
    stream.WriteBool(*val.ShowDetails)
    }
    stream.WriteMore()
    stream.WriteObjectField(`theme`)
    stream.WriteString(val.Theme)
    stream.WriteMore()
    stream.WriteObjectField(`refresh_frequency`)
    stream.WriteString(val.RefreshFrequency)
    stream.WriteMore()
}
