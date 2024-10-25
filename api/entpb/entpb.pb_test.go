package entpb

import (
	"encoding/json"
	"testing"
)

func TestKeyValueStore_String(t *testing.T) {
	store := KeyValueStore{
		Id:        1,
		Key:       "apple",
		Value:     []byte("red"),
		CreatedAt: 123,
		UpdatedAt: 456,
	}
	raw, err := json.Marshal(&store)
	if err != nil {
		t.Fatalf("json marshal error: %v", err)
	}
	t.Logf("%s\n", raw)
	restore := &KeyValueStore{}
	err = json.Unmarshal(raw, restore)
	if err != nil {
		t.Fatalf("json unmarshal error: %v", err)
	}
	if store.Id != restore.Id {
		t.Fatalf("store.Id != restore.Id")
	}
	if store.Key != restore.Key {
		t.Fatalf("store.Key != restore.Key")
	}
	if string(store.Value) != string(restore.Value) {
		t.Fatalf("store.Value != restore.Value")
	}
	if store.CreatedAt != restore.CreatedAt {
		t.Fatalf("store.CreatedAt != restore.CreatedAt")
	}
}
