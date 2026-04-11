# 🧪 TESTS Y BENCHMARKS - VALIDACIÓN DE OPTIMIZACIONES

## 📊 Criterios de Éxito

Después de cada fase, verifica que estos métricas mejoren:

```
┌─────────────────────────────────────────────────────────────┐
│                    MÉTRICAS DE ÉXITO                       │
├─────────────────────────────────────────────────────────────┤
│ Métrica                  │ Antes    │ Después  │ Target    │
├──────────────────────────┼──────────┼──────────┼───────────┤
│ Búsqueda simple          │ 20s      │ 2-4s     │ < 5s ✅   │
│ Búsqueda repetida        │ 20s      │ <1ms     │ <2ms ✅   │
│ Búsqueda exacta          │ 20s      │ 50-100ms │ <200ms ✅ │
│ Metadata (25 reqs)       │ 5-10s    │ 1-2s     │ <3s ✅    │
│ Total install flow       │ 25-45s   │ 4-6s     │ <10s ✅   │
│ Índice en memoria        │ ∞        │ 3-5MB    │ <10MB ✅  │
│ Caché en disco           │ N/A      │ 2-5MB    │ <10MB ✅  │
└──────────────────────────┴──────────┴──────────┴───────────┘
```

---

## 🧪 TESTS UNITARIOS

### Para Caché Local (`requirements/cache_test.go`)

```go
package requirements

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCacheLoadAndSave(t *testing.T) {
	// Setup: crea directorio temporal
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "test_cache.json.gz")

	cache := &IndexCache{cachePath: cachePath}
	testPackages := []string{
		"numpy", "pandas", "requests", "flask",
	}

	// Test 1: Guarda datos
	t.Run("SaveToDisk", func(t *testing.T) {
		err := cache.saveToDisk(testPackages)
		if err != nil {
			t.Fatalf("Error guardando caché: %v", err)
		}

		// Verifica que archivo existe
		if _, err := os.Stat(cachePath); err != nil {
			t.Fatalf("Archivo caché no creado: %v", err)
		}

		// Verifica tamaño (debe estar comprimido)
		info, _ := os.Stat(cachePath)
		if info.Size() >= 1000 {
			t.Fatalf("Caché comprimido demasiado grande: %d bytes", info.Size())
		}
	})

	// Test 2: Carga datos
	t.Run("LoadFromDisk", func(t *testing.T) {
		loaded, err := cache.loadFromDisk()
		if err != nil {
			t.Fatalf("Error cargando caché: %v", err)
		}

		// Verifica que datos coinciden
		if len(loaded) != len(testPackages) {
			t.Fatalf("Datos perdidos. Esperado %d, obtenido %d",
				len(testPackages), len(loaded))
		}

		for i, pkg := range testPackages {
			if loaded[i] != pkg {
				t.Fatalf("Dato corrupto en índice %d", i)
			}
		}
	})

	// Test 3: Invalidación por edad
	t.Run("CacheExpiration", func(t *testing.T) {
		// Cambia fecha de modificación al pasado
		pastTime := time.Now().AddDate(0, 0, -8)  // 8 días atrás
		os.Chtimes(cachePath, pastTime, pastTime)

		// Intenta cargar - debe fallar por expiración
		_, err := cache.loadFromDisk()
		if err == nil {
			t.Fatal("Caché expirado no fue rechazado")
		}
	})
}

func BenchmarkCacheLoad(b *testing.B) {
	// Setup
	tmpDir := b.TempDir()
	cachePath := filepath.Join(tmpDir, "bench.json.gz")
	testPkgs := generateTestPackages(100000)

	cache := &IndexCache{cachePath: cachePath}
	cache.saveToDisk(testPkgs)

	// Benchmark
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.loadFromDisk()
	}
}

// Genera lista de test
func generateTestPackages(count int) []string {
	pkgs := make([]string, count)
	for i := 0; i < count; i++ {
		pkgs[i] = "package-" + string(rune(i))
	}
	return pkgs
}
```

**Cómo ejecutar:**
```bash
go test -v ./internal/requirements -run TestCache
go test -bench BenchmarkCacheLoad -benchmem ./internal/requirements
```

---

### Para Índice Trie (`requirements/trie_test.go`)

```go
package requirements

import (
	"strings"
	"testing"
	"time"
)

func TestTrieSearchOptimized(t *testing.T) {
	packages := []string{
		"requests",
		"requests-aws4auth",
		"requests-mock",
		"requests-ntlm",
		"requestsexceptions",
		"numpy",
		"pandas",
		"flask",
	}

	trie := NewPackageTrie(packages)

	tests := []struct {
		query    string
		expected int
		name     string
	}{
		{"requests", 5, "Prefix match"},         // 5 empiezan con "requests"
		{"req", 5, "Short prefix"},
		{"numpy", 1, "Exact match"},
		{"pan", 1, "Prefix pandas"},
		{"xyznonexistent", 0, "No match"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := trie.SearchOptimized(tt.query, 100)

			// Verifica cantidad
			if len(results) != tt.expected {
				t.Errorf("Query %q: esperado %d resultados, obtenido %d",
					tt.query, tt.expected, len(results))
			}

			// Verifica que todos empiezan con query
			for _, result := range results {
				if !strings.HasPrefix(strings.ToLower(result),
					strings.ToLower(tt.query)) {
					t.Errorf("Resultado %q no empieza con %q", result, tt.query)
				}
			}
		})
	}
}

func BenchmarkTrieSearchVsFuzzy(b *testing.B) {
	// Setup: genera índice grande
	packages := make([]string, 100000)
	for i := 0; i < 100000; i++ {
		packages[i] = "package-" + string(rune(i))
	}

	// Agrega algunos reales
	packages = append(packages, "requests", "numpy", "pandas", "flask")
	trie := NewPackageTrie(packages)

	query := "req"

	// Benchmark Trie
	b.Run("TrieSearch", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			trie.SearchOptimized(query, 25)
		}
	})

	// Benchmark Fuzzy (fallback)
	b.Run("FuzzySearch", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			trie.SearchFuzzy(query, 25)
		}
	})
}

func TestTrieExactMatch(t *testing.T) {
	packages := []string{"requests", "requests-aws4auth", "numpy"}
	trie := NewPackageTrie(packages)

	// Búsqueda exacta insensible a caso
	results := trie.SearchOptimized("REQUESTS", 10)

	if len(results) == 0 {
		t.Fatal("Exact match case-insensitive falló")
	}

	if results[0] != "requests" {
		t.Errorf("Esperado 'requests', obtenido %q", results[0])
	}
}
```

**Cómo ejecutar:**
```bash
go test -v ./internal/requirements -run TestTrie
go test -bench BenchmarkTrieSearchVsFuzzy -benchmem ./internal/requirements
```

---

### Para Search Cache (`requirements/search_cache_test.go`)

```go
package requirements

import (
	"testing"
	"time"
)

func TestSearchCacheHitMiss(t *testing.T) {
	cache := NewSearchCache(10)

	query1 := "numpy"
	results1 := []Result{
		{Name: "numpy", Version: "1.24.0", Description: "Numeric library"},
	}

	// Test 1: Cache miss
	t.Run("CacheMiss", func(t *testing.T) {
		_, ok := cache.Get(query1)
		if ok {
			t.Fatal("Cache no debería tener entrada para nueva query")
		}
	})

	// Test 2: Cache set y hit
	t.Run("CacheSetAndHit", func(t *testing.T) {
		cache.Set(query1, results1)

		// Intenta recuperar
		retrieved, ok := cache.Get(query1)
		if !ok {
			t.Fatal("Cache miss para entrada previamente guardada")
		}

		// Verifica datos
		if len(retrieved) != len(results1) {
			t.Fatal("Cache corrupted")
		}

		if retrieved[0].Name != "numpy" {
			t.Fatalf("Valor incorrecto: %v", retrieved[0])
		}
	})

	// Test 3: Tamaño máximo
	t.Run("CacheMaxSize", func(t *testing.T) {
		cache2 := NewSearchCache(3)

		// Agrega 4 entradas
		for i := 0; i < 4; i++ {
			query := "query-" + string(rune(i))
			cache2.Set(query, []Result{})
		}

		// Stats debe mostrar máximo 3
		stats := cache2.Stats()
		size := stats["size"].(int)
		if size > 3 {
			t.Fatalf("Cache excedió tamaño máximo: %d > 3", size)
		}
	})

	// Test 4: Invalidación por tiempo
	t.Run("CacheExpiration", func(t *testing.T) {
		cache3 := NewSearchCache(10)
		cache3.Set("temporal", []Result{})

		// Debe estar presente
		_, ok := cache3.Get("temporal")
		if !ok {
			t.Fatal("Entrada no debería expirar inmediatamente")
		}

		// Espera a que se "expire" (en test usamos mock del tiempo)
		// En producción espera 30 minutos
	})
}

func BenchmarkSearchCache(b *testing.B) {
	cache := NewSearchCache(100)
	query := "requests"
	results := []Result{
		{Name: "requests", Version: "2.31.0"},
	}

	cache.Set(query, results)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Get(query)
	}
}
```

---

## 🚀 BENCHMARKS INTEGRALES

### Script: `benchmark.sh` (para ejecutar en terminal)

```bash
#!/bin/bash

echo "🧪 BENCHMARKS PIPNEST REQUIREMENTS"
echo "=================================="
echo

# Compila
echo "📦 Compilando..."
go build -o pipnest ./cmd/...
echo

echo "⏱️  BENCHMARKS"
echo

# Benchmark de búsqueda
echo "1. Búsqueda Caché"
go test -bench BenchmarkCacheLoad -benchmem ./internal/requirements

echo
echo "2. Búsqueda Trie vs Fuzzy"
go test -bench BenchmarkTrieSearchVsFuzzy -benchmem ./internal/requirements

echo
echo "3. Search Cache"
go test -bench BenchmarkSearchCache -benchmem ./internal/requirements

echo
echo "4. HTTP Paralela (si existe)"
go test -bench BenchmarkFetchMetadata -benchmem ./internal/requirements 2>/dev/null || echo "Benchmark no encontrado"

echo
echo "✅ Benchmarks completados"
```

**Cómo ejecutar:**
```bash
chmod +x benchmark.sh
./benchmark.sh
```

---

## 📋 CHECKLIST DE VALIDACIÓN

Después de implementar cada fase, verifica:

### **Fase 1: Caché Local**
```
[ ] Archivo caché se crea en ~/.cache/pipnest/
[ ] Primera búsqueda toma ~30s (descarga índice)
[ ] Segunda búsqueda toma <100ms (desde caché)
[ ] Búsquedas subsiguientes son <100ms
[ ] go test ./internal/requirements pasa sin errores
[ ] BenchmarkCacheLoad muestra >10000 ops/sec
```

### **Fase 2: Búsqueda Paralela**
```
[ ] fetchPackageMetadataParallel() está implementada
[ ] 5 goroutines ejecutan en paralelo
[ ] 25 requests toman 1-2s (vs 5-10s antes)
[ ] No hay errores de concurrencia (race detector)
[ ] Memoria no explota
```

### **Fase 3: Índice Trie**
```
[ ] TrieNode structure implementada
[ ] Búsqueda exacta retorna en <100ms
[ ] Búsqueda prefix retorna en <50ms
[ ] SearchOptimized prioriza exacta > prefix > fuzzy
[ ] TestTrieSearchVsFuzzy pasa
[ ] BenchmarkTrieSearchVsFuzzy muestra >1000x mejora en prefix
```

### **Fase 4: Search Cache**
```
[ ] SearchCache estructura implementada
[ ] Cache hit retorna <1ms
[ ] Cache miss busca normalmente
[ ] Máximo 50 entradas en caché
[ ] TestSearchCacheHitMiss pasa
```

### **Fase 5: Timeouts Reducidos**
```
[ ] searchTimeout: 5s
[ ] listTimeout: 5s
[ ] installTimeout: 20s
[ ] App no se cuelga con conexión lenta
[ ] Tests de timeout pasan
```

---

## 🎯 SCRIPT DE VALIDACIÓN MANUAL

### `test_performance.go` (para ejecutar directamente)

```go
package main

import (
	"context"
	"fmt"
	"time"

	"pipnest/internal/requirements"
)

func main() {
	fmt.Println("🧪 TEST DE RENDIMIENTO PIPNEST")
	fmt.Println("==============================\n")

	// Test 1: Caché
	fmt.Println("📦 Test 1: Caché Local")
	testCache()

	fmt.Println("\n✨ Test 2: Búsqueda Trie")
	testTrie()

	fmt.Println("\n💾 Test 3: Search Cache")
	testSearchCache()

	fmt.Println("\n✅ Todos los tests completados")
}

func testCache() {
	cache := requirements.NewIndexCache()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := time.Now()
	packages, err := cache.LoadOrFetch(ctx)
	elapsed := time.Since(start)

	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		return
	}

	fmt.Printf("✅ Índice cargado: %d paquetes en %.2fs\n", len(packages), elapsed.Seconds())

	// Segunda carga debe ser más rápida
	start = time.Now()
	_, _ = cache.LoadOrFetch(ctx)
	elapsed2 := time.Since(start)

	fmt.Printf("✅ Segunda carga: %.2fms (caché)\n", elapsed2.Seconds()*1000)

	if elapsed2 > 1*time.Second {
		fmt.Printf("⚠️  Advertencia: caché lenta, esperaba <100ms\n")
	}
}

func testTrie() {
	packages := []string{
		"requests", "requests-aws4auth", "numpy",
		"pandas", "flask", "django",
	}

	trie := requirements.NewPackageTrie(packages)

	tests := []struct {
		query string
	}{
		{"req"},
		{"requests"},
		{"np"},
		{"xyz"},
	}

	for _, tt := range tests {
		start := time.Now()
		results := trie.SearchOptimized(tt.query, 25)
		elapsed := time.Since(start)

		fmt.Printf("Query %q: %d resultados en %.2fms\n",
			tt.query, len(results), elapsed.Seconds()*1000)
	}
}

func testSearchCache() {
	cache := requirements.NewSearchCache(50)

	// Simula búsqueda
	results := []requirements.Result{
		{Name: "test", Version: "1.0.0", Description: "test pkg"},
	}

	cache.Set("test-query", results)

	// Cache hit
	start := time.Now()
	retrieved, ok := cache.Get("test-query")
	elapsed := time.Since(start)

	fmt.Printf("✅ Cache hit: %v en %.2fμs\n", ok, elapsed.Microseconds())

	// Cache miss
	start = time.Now()
	_, ok = cache.Get("nonexistent")
	elapsed = time.Since(start)

	fmt.Printf("✅ Cache miss: %v en %.2fμs\n", ok, elapsed.Microseconds())
}
```

**Cómo ejecutar:**
```bash
go run test_performance.go
```

---

## 📊 COMPARATIVA DE RESULTADOS

### Usando BenchBench Tool

```bash
# Instala (si no tienes)
go install golang.org/x/perf/cmd/benchstat@latest

# Ejecuta benchmark ANTES de cambios
go test -bench . -benchmem ./internal/requirements > before.txt

# Implementa cambios

# Ejecuta benchmark DESPUÉS
go test -bench . -benchmem ./internal/requirements > after.txt

# Compara
benchstat before.txt after.txt
```

---

## ✅ CRITERIOS DE ACEPTACIÓN

Tu implementación es exitosa si:

1. **Performance:**
   - [ ] Búsqueda simple: < 5 segundos
   - [ ] Búsqueda repetida: < 1 milisegundo
   - [ ] Total install: < 10 segundos

2. **Memoria:**
   - [ ] Índice en memoria: < 10MB
   - [ ] Caché en disco: < 10MB
   - [ ] No hay memory leaks (go test -run TestMemory)

3. **Correctitud:**
   - [ ] Todos los tests pasan: `go test ./...`
   - [ ] Race detector limpio: `go test -race ./...`
   - [ ] Compilación sin warnings

4. **UX:**
   - [ ] App no se congela (asyncronía)
   - [ ] Feedback visual mientras carga
   - [ ] Errores manejados gracefully

