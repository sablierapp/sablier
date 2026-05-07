# pkg/template

This package contains forked copies of Go's `text/template`, `html/template`, and `internal/fmtsort` stdlib packages, patched to avoid disabling the linker's **method dead code elimination** optimization.

## Why

The Go linker can eliminate unreachable code (dead code elimination, DCE). However, DCE for methods is disabled whenever the linker sees a call to `reflect.Value.MethodByName` with a non-constant argument — because the linker cannot know at build time which methods might be invoked.

`text/template`'s `evalField` function contains exactly such a call:

```go
// text/template/exec.go — REMOVED in this fork
if method := ptr.MethodByName(fieldName); method.IsValid() {
    return s.evalCall(dot, method, false, node, fieldName, args, final)
}
```

This single call forces the linker to keep **every exported method of every reachable type** in the binary.

Removing it means templates can no longer call methods on data objects via `{{ .SomeMethod }}`. Only struct **field** access and **range** over slices/maps are supported. This is sufficient for Sablier's use case, which only accesses plain struct fields.

## What changed

The only functional change is in `text/exec.go`: the 9-line block that calls `ptr.MethodByName(fieldName)` is removed. All other code is identical to the Go stdlib version pinned below.

This is the same approach used by [DataDog's agent](https://github.com/DataDog/datadog-agent/tree/main/pkg/template).

There is an open Go proposal ([golang/go#72895](https://github.com/golang/go/issues/72895)) to allow statically disabling method calls in templates. Once that lands, this fork can be removed.

## Go version

The code is from **Go 1.26.0**.

## Updating

When upgrading Go:

```bash
GOROOT=$(go env GOROOT)
DEST=pkg/template

# Re-copy source files
for f in doc.go exec.go funcs.go helper.go option.go template.go; do
  cp $GOROOT/src/text/template/$f $DEST/text/$f
done

for f in attr.go attr_string.go content.go context.go css.go delim_string.go doc.go \
          element_string.go error.go escape.go html.go js.go jsctx_string.go \
          state_string.go template.go transition.go url.go urlpart_string.go; do
  cp $GOROOT/src/html/template/$f $DEST/html/$f
done

cp $GOROOT/src/internal/fmtsort/sort.go $DEST/internal/fmtsort/sort.go

# Fix imports
sed -i 's|"internal/fmtsort"|"github.com/sablierapp/sablier/pkg/template/internal/fmtsort"|g' $DEST/text/*.go
sed -i 's|"text/template"|"github.com/sablierapp/sablier/pkg/template/text"|g' $DEST/html/*.go

# Remove internal/godebug import and its unused variable from escape.go
# (the jstmpllitinterp godebug knob is deprecated; the variable is declared but never read)
sed -i '/"internal\/godebug"/d' $DEST/html/escape.go
sed -i '/debugAllowActionJSTmpl/d' $DEST/html/escape.go

# Re-apply the no-method patch to text/exec.go (see no-method.patch)
patch -p1 < pkg/template/no-method.patch
```
