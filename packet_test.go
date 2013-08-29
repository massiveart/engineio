package engineio

import (
	"bytes"
	"testing"
)

func TestPacketDecode(t *testing.T) {
	data := []byte("8:4aaaaaaa")
	packets, err := decode(data)
	if err != nil {
		t.Fatalf("decode 1: %v", err)
	}
	if len(packets) != 1 {
		t.Fatalf("decode 1: expect 1 packet, got %d", len(packets))
	}
	if packets[0].Type != "4" {
		t.Fatalf("decode 1: expect packet type \"4\", got %q", packets[0].Type)
	}
	if bytes.Compare(packets[0].Data, []byte("aaaaaaa")) != 0 {
		t.Fatalf("decode 1: expect packet data \"aaaaaaa\", got %q", packets[0].Data)
	}

	data = []byte("8:4aaaaaaa10:4xxxxxxxxx4:2bbb")
	packets, err = decode(data)
	if err != nil {
		t.Fatalf("decode 3: %v", err)
	}
	if len(packets) != 3 {
		t.Fatalf("decode 3: expect 3 packets, got %d", len(packets))
	}

	for i, ty := range []string{"4", "4", "2"} {
		if packets[i].Type != ty {
			t.Fatalf("decode: expect packet type %q, got %q", ty, packets[i].Type)
		}
	}
	for i, ty := range []string{"aaaaaaa", "xxxxxxxxx", "bbb"} {
		if bytes.Compare(packets[0].Data, []byte("aaaaaaa")) != 0 {
			t.Fatalf("decode: expect packet data %q, got %q", ty, packets[i].Data)
		}
	}

	data = []byte(`15:4{"test":"aaa"}`)
	packets, err = decode(data)
	if err != nil {
		t.Fatalf("decode 4: %v", err)
	}

	if len(packets) != 1 {
		t.Fatalf("decode 4: expect 1 packet, got %d", len(packets))
	}
	if packets[0].Type != "4" {
		t.Fatalf("decode 4: expect packet type \"4\", got %q", packets[0].Type)
	}
	if bytes.Compare(packets[0].Data, []byte(`{"test":"aaa"}`)) != 0 {
		t.Fatalf("decode 4: expect packet data \"{\"test\":\"aaa\"}\", got %q", packets[0].Data)
	}
}

func TestInvalidPacket(t *testing.T) {
	data := []byte("8:4aaa")
	_, err := decode(data)
	if err == nil {
		t.Fatalf("invalid 1: expected non nil err")
	}

	data = []byte("2:4aaaaaaaaaa")
	_, err = decode(data)
	if err == nil {
		t.Fatalf("invalid 2: expected non nil err")
	}

	data = []byte("8:4aaa34:4aaa")
	_, err = decode(data)
	if err == nil {
		t.Fatalf("invalid 3: expected non nil err")
	}

	data = []byte("3:4::::::::::")
	_, err = decode(data)
	if err == nil {
		t.Fatalf("invalid 4: expected non nil err")
	}

	data = []byte("3:4")
	_, err = decode(data)
	if err == nil {
		t.Fatalf("invalid 5: expected non nil err")
	}
}
