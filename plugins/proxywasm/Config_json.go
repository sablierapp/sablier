package main

import jsoniter "github.com/json-iterator/tinygo"

type Config_json struct {
}
func (json Config_json) Type() interface{} {
  var val Config
  return val
}
func (json Config_json) Unmarshal(iter *jsoniter.Iterator, out interface{}) {
  Config_json_unmarshal(iter, out.(*Config))
}
func (json Config_json) Marshal(stream *jsoniter.Stream, val interface{}) {
  Config_json_marshal(stream, val.(Config))
}
func Config_json_unmarshal(iter *jsoniter.Iterator, out *Config) {
  more := iter.ReadObjectHead()
  for more {
    field := iter.ReadObjectField()
    if !Config_json_unmarshal_field(iter, field, out) {
      iter.Skip()
    }
    more = iter.ReadObjectMore()
  }
}
func Config_json_unmarshal_field(iter *jsoniter.Iterator, field string, out *Config) bool {
  switch {
  case field == `sablier_url`:
    iter.ReadString(&(*out).SablierURL)
    return true
  case field == `cluster`:
    iter.ReadString(&(*out).Cluster)
    return true
  case field == `names`:
    Config_array1_json_unmarshal(iter, &(*out).Names)
    return true
  case field == `group`:
    iter.ReadString(&(*out).Group)
    return true
  case field == `session_duration`:
    iter.ReadString(&(*out).SessionDuration)
    return true
  case field == `dynamic`:
    Config_ptr2_json_unmarshal(iter, &(*out).Dynamic)
    return true
  case field == `blocking`:
    Config_ptr3_json_unmarshal(iter, &(*out).Blocking)
    return true
  }
  return false
}
func Config_array1_json_unmarshal (iter *jsoniter.Iterator, out *[]string) {
  i := 0
  val := *out
  more := iter.ReadArrayHead()
  for more {
    if i == len(val) {
      val = append(val, make([]string, 4)...)
    }
    iter.ReadString(&val[i])
    i++
    more = iter.ReadArrayMore()
  }
  if i == 0 {
    *out = []string{}
  } else {
    *out = val[:i]
  }
}
func Config_ptr2_json_unmarshal (iter *jsoniter.Iterator, out **DynamicConfiguration) {
    var val DynamicConfiguration
    DynamicConfiguration_json_unmarshal(iter, &val)
    if iter.Error == nil {
      *out = &val
    }
}
func Config_ptr3_json_unmarshal (iter *jsoniter.Iterator, out **BlockingConfiguration) {
    var val BlockingConfiguration
    BlockingConfiguration_json_unmarshal(iter, &val)
    if iter.Error == nil {
      *out = &val
    }
}
func Config_json_marshal(stream *jsoniter.Stream, val Config) {
    stream.WriteObjectHead()
    Config_json_marshal_field(stream, val)
    stream.WriteObjectTail()
}
func Config_json_marshal_field(stream *jsoniter.Stream, val Config) {
    stream.WriteObjectField(`sablier_url`)
    stream.WriteString(val.SablierURL)
    stream.WriteMore()
    stream.WriteObjectField(`cluster`)
    stream.WriteString(val.Cluster)
    stream.WriteMore()
    stream.WriteObjectField(`names`)
    Config_array4_json_marshal(stream, val.Names)
    stream.WriteMore()
    stream.WriteObjectField(`group`)
    stream.WriteString(val.Group)
    stream.WriteMore()
    stream.WriteObjectField(`session_duration`)
    stream.WriteString(val.SessionDuration)
    stream.WriteMore()
    stream.WriteObjectField(`dynamic`)
    if val.Dynamic == nil {
       stream.WriteNull()
    } else {
    DynamicConfiguration_json_marshal(stream, *val.Dynamic)
    }
    stream.WriteMore()
    stream.WriteObjectField(`blocking`)
    if val.Blocking == nil {
       stream.WriteNull()
    } else {
    BlockingConfiguration_json_marshal(stream, *val.Blocking)
    }
    stream.WriteMore()
}
func Config_array4_json_marshal (stream *jsoniter.Stream, val []string) {
  if len(val) == 0 {
    stream.WriteEmptyArray()
  } else {
    stream.WriteArrayHead()
    for i, elem := range val {
      if i != 0 { stream.WriteMore() }
    stream.WriteString(elem)
    }
    stream.WriteArrayTail()
  }
}
