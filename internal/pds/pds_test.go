package pds

import (
	"math/rand"
	"testing"
)

func checkVec(t *testing.T, v *Vec, model []any) {
	t.Helper()
	if v.Len() != len(model) {
		t.Fatalf("len %d, want %d", v.Len(), len(model))
	}
	for i, x := range model {
		if got := v.Get(i); got != x {
			t.Fatalf("Get(%d) = %v, want %v", i, got, x)
		}
	}
	got := v.ToSlice()
	if len(got) != len(model) {
		t.Fatalf("ToSlice len %d, want %d", len(got), len(model))
	}
	for i := range got {
		if got[i] != model[i] {
			t.Fatalf("ToSlice[%d] = %v, want %v", i, got[i], model[i])
		}
	}
}

func TestVecPushGetAcrossLevels(t *testing.T) {
	v := Empty()
	var model []any
	for i := 0; i < 40000; i++ {
		v = v.Push(i)
		model = append(model, i)
		if i < 70 || i%997 == 0 || i == 1055 || i == 1056 || i == 32799 || i == 32800 {
			checkVec(t, v, model)
		}
	}
	checkVec(t, v, model)
}

func TestVecPersistence(t *testing.T) {
	a := Empty()
	for i := 0; i < 2000; i++ {
		a = a.Push(i)
	}
	b := a.Set(5, "x").Set(1999, "y").Push("z")
	if a.Get(5) != 5 || a.Get(1999) != 1999 || a.Len() != 2000 {
		t.Fatal("Set/Push mutated the receiver")
	}
	if b.Get(5) != "x" || b.Get(1999) != "y" || b.Get(2000) != "z" || b.Len() != 2001 {
		t.Fatal("derived vector is wrong")
	}
}

func TestVecRandomOps(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for round := 0; round < 30; round++ {
		n := rng.Intn(5000)
		model := make([]any, n)
		for i := range model {
			model[i] = rng.Int()
		}
		v := FromSlice(model)
		checkVec(t, v, model)
		for k := 0; k < 300; k++ {
			switch rng.Intn(3) {
			case 0:
				x := rng.Int()
				v = v.Push(x)
				model = append(model[:len(model):len(model)], x)
			case 1:
				if len(model) > 0 {
					i, x := rng.Intn(len(model)), rng.Int()
					v = v.Set(i, x)
					model = append([]any(nil), model...)
					model[i] = x
				}
			case 2:
				if len(model) > 0 {
					a := rng.Intn(len(model) + 1)
					b := a + rng.Intn(len(model)-a+1)
					v = v.Slice(a, b)
					model = model[a:b:b]
				}
			}
		}
		checkVec(t, v, model)
	}
}

func TestVecConcat(t *testing.T) {
	for _, sizes := range [][2]int{{0, 0}, {0, 40}, {40, 0}, {31, 33}, {1000, 5}, {5, 1000}, {2000, 2000}} {
		var am, bm []any
		for i := 0; i < sizes[0]; i++ {
			am = append(am, i)
		}
		for i := 0; i < sizes[1]; i++ {
			bm = append(bm, -i-1)
		}
		checkVec(t, FromSlice(am).Concat(FromSlice(bm)), append(append([]any{}, am...), bm...))
	}
}

func testCfg() *Config {
	return &Config{
		Hash:  func(k any) uint64 { return uint64(k.(int)) % 97 }, // many collisions on purpose
		Equal: func(a, b any) bool { return a == b },
	}
}

func TestMapAgainstModel(t *testing.T) {
	rng := rand.New(rand.NewSource(2))
	m := NewMap(testCfg())
	model := map[int]int{}
	var order []int
	for step := 0; step < 20000; step++ {
		k := rng.Intn(600)
		if rng.Intn(3) == 0 {
			m = m.Remove(k)
			if _, ok := model[k]; ok {
				delete(model, k)
				for i, o := range order {
					if o == k {
						order = append(order[:i:i], order[i+1:]...)
						break
					}
				}
			}
		} else {
			v := rng.Int()
			m = m.Put(k, v)
			if _, ok := model[k]; !ok {
				order = append(order, k)
			}
			model[k] = v
		}
		if step%500 == 0 || step == 19999 {
			if m.Len() != len(model) {
				t.Fatalf("len %d, want %d", m.Len(), len(model))
			}
			keys := m.Keys()
			if len(keys) != len(order) {
				t.Fatalf("keys %d, want %d", len(keys), len(order))
			}
			for i, k := range keys {
				if k != order[i] {
					t.Fatalf("order[%d] = %v, want %v", i, k, order[i])
				}
			}
			for k, v := range model {
				if got, ok := m.Get(k); !ok || got != v {
					t.Fatalf("Get(%d) = %v,%v want %v", k, got, ok, v)
				}
			}
			if m.Has(100000) {
				t.Fatal("phantom key")
			}
		}
	}
}

func TestMapPersistence(t *testing.T) {
	a := NewMap(testCfg()).Put(1, "a").Put(2, "b")
	b := a.Put(3, "c").Put(1, "z").Remove(2)
	if v, _ := a.Get(1); v != "a" || a.Len() != 2 || !a.Has(2) {
		t.Fatal("receiver was mutated")
	}
	if v, _ := b.Get(1); v != "z" || b.Len() != 2 || b.Has(2) || !b.Has(3) {
		t.Fatal("derived map is wrong")
	}
}

func BenchmarkVecPush(b *testing.B) {
	for i := 0; i < b.N; i++ {
		v := Empty()
		for j := 0; j < 10000; j++ {
			v = v.Push(j)
		}
	}
}

func BenchmarkMapPut(b *testing.B) {
	cfg := &Config{Hash: func(k any) uint64 { return uint64(k.(int)) * 0x9E3779B97F4A7C15 }, Equal: func(a, b any) bool { return a == b }}
	for i := 0; i < b.N; i++ {
		m := NewMap(cfg)
		for j := 0; j < 10000; j++ {
			m = m.Put(j, j)
		}
	}
}
