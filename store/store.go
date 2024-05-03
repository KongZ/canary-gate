package store

type Store interface {
	GateOpen(key string)
	GateClose(key string)
	IsGateOpen(key string) bool
}
