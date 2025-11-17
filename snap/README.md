# snap

Inline snapshot testing for Go inspired by [TigerBeetle: Snapshot Testing For The Masses](https://tigerbeetle.com/blog/2024-05-14-snapshot-testing-for-the-masses/)

## Idea

1. Your code generates **actual** output (e.g., logs, rendered text, formatted structs).
2. You place an empty or placeholder **expected** snapshot in the test.
3. Running the test with update enabled rewrites that snapshot with the real output.

## Helper

```go
func check(t *testing.T, actual string, snapshot snap.Snapshot) {
	t.Helper()
	if !snapshot.Diff(actual) {
		t.Fatal("Snapshot mismatch")
	}
}
```

## Usage


```go
lgr.Warn().Msg("hello")
actual := LogOutputBuffer.String()
check(t, actual, snap.New(``))
```

First run with update:

```go
check(t, actual, snap.New(``).Update())
```

The call site is rewritten:

```go
// Output itself has a newline at the end
check(t, actual, snap.New(`2000-01-31T23:59:59Z|WRN|hello|
`))
```

Now future runs compare `actual` vs `expected`.

## Update modes

Update just this snapshot:

```go
snap.New(``).Update()
```

Update all:

```sh
SNAPSHOT_UPDATE_ALL=1 go test ./...
```

## Notes

- `actual` is whatever your code produced.
- `expected` is the inline backtick string in `snap.New(...)`.
- One snapshot per line.
- Backticks must already be inside `snap.New(``)`

## Example

Refer to `golang_snacks/tree/main/itlog/unit_test.go`.
