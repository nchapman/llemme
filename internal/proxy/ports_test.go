package proxy

import (
	"testing"
)

func TestPortAllocator(t *testing.T) {
	allocator := NewPortAllocator(59000, 59005)

	// Should be able to allocate ports
	port1, err := allocator.Allocate()
	if err != nil {
		t.Fatalf("First allocation failed: %v", err)
	}
	if port1 < 59000 || port1 > 59005 {
		t.Errorf("Port %d outside range 59000-59005", port1)
	}

	// Should track allocated ports
	if !allocator.IsAllocated(port1) {
		t.Errorf("Port %d should be marked as allocated", port1)
	}

	// Allocate more ports
	port2, _ := allocator.Allocate()
	_, _ = allocator.Allocate() // port3

	if allocator.AllocatedCount() != 3 {
		t.Errorf("AllocatedCount() = %d, want 3", allocator.AllocatedCount())
	}

	// Release a port
	allocator.Release(port2)

	if allocator.IsAllocated(port2) {
		t.Errorf("Port %d should not be allocated after release", port2)
	}

	if allocator.AllocatedCount() != 2 {
		t.Errorf("AllocatedCount() = %d, want 2", allocator.AllocatedCount())
	}

	// Should be able to reallocate the released port
	port4, err := allocator.Allocate()
	if err != nil {
		t.Fatalf("Allocation after release failed: %v", err)
	}

	// The reallocated port should be the released one or another available port
	if port4 < 59000 || port4 > 59005 {
		t.Errorf("Reallocated port %d outside range", port4)
	}
}

func TestPortAllocatorExhaustion(t *testing.T) {
	// Very small range for testing exhaustion
	allocator := NewPortAllocator(59100, 59101)

	// Allocate all available ports
	_, err1 := allocator.Allocate()
	_, err2 := allocator.Allocate()

	if err1 != nil || err2 != nil {
		t.Skip("Ports 59100-59101 not available for testing")
	}

	// Third allocation should fail (range exhausted)
	_, err := allocator.Allocate()
	if err == nil {
		t.Error("Expected error when port range exhausted")
	}
}
