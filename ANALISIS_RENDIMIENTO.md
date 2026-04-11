# 📊 ANÁLISIS DE RENDIMIENTO - PIPNEST REQUIREMENTS

## 🐌 PROBLEMAS IDENTIFICADOS

### **1. BÚSQUEDA EN MODAL (Principal cuello de botella)**

**Problema:** `searchSuggestionsCmd()` es demasiado lenta
- Timeout: 20 segundos (¡muy conservador!)
- Requiere cargar TODO el índice de PyPI en memoria (~450K paquetes)
- Fuzzy scoring O(n) para cada carácter digitado
- Usuario ve delay perceptible al escribir

**Impacto:**
```
Usuario escribe "requests"
  ↓ keystroke 'r'
  ↓ 20s timeout espera
  ↓ fuzzyScore() en 450K paquetes
  ↓ Espera incómoda
```

---

### **2. CARGA DEL ÍNDICE PyPI**

**Problema:** `loadPackageIndex()` descarga HTML de https://pypi.org/simple/ (~5-10 MB)
```
fetchPackageIndex():
  ├─ GET https://pypi.org/simple/
  ├─ Parsea 450K+ <a href="...">
  ├─ Extrae nombres
  └─ PRIMERA VEZ: puede tardar 15-30 segundos
```

**Impacto:**
- La primera búsqueda es lentísima
- No hay feedback visual al usuario (parece congelado)
- Si la red falla, sin respaldo

---

### **3. BÚSQUEDA DE METADATA INNECESARIA**

**Problema:** `Search()` en pantalla inicial hace 2 HTTP calls por cada top-25

```go
fetchPackageMetadata() {
  for _, result := range topResults[:25] {
    GET https://pypi.org/pypi/<name>/json  // ← 25 requests HTTP
  }
}
```

**Impacto:**
- 25 requests secuenciales en serie
- Si falla uno, no hay retry
- Timeout total: 20s x 25 = muy fragile

---

### **4. FUZZY SCORING INEFICIENTE**

**Problema:** Algoritmo O(n) en cada keystroke

```go
for _, pkg := range allPackages { // 450K iteraciones
  score := fuzzyScore(query, pkg)
}
```

**Impacto:**
- No hay **early termination** en queries comunes
- No hay **cache del índice** dentro de la sesión
- Recalcula scores incluso para queries iguales

---

### **5. TIMEOUT CONSERVADOR**

**Problema:** Timeouts muy generosos para contextos rápidos

```go
searchSuggestionsCmd:  20 segundos  ← demasiado?
loadInstalledCmd:      30 segundos  ← demasiado?
```

**Realidad:**
- Pip list en venv pequeño: <1 segundo
- PyPI search: 2-4 segundos
- Timeout largo = usuario espera más

---

### **6. SIN PAGINACIÓN EN RESULTADOS**

**Problema:** Siempre retorna TOP 25 aunque usuario necesite ver más

```
Usuario selecciona resultado 25
  ↓
¿Hay más? No hay forma de paginar
  ↓
Frustración
```

---

### **7. BÚSQUEDA EXACTA NO OPTIMIZADA**

**Problema:** No se aprovecha búsqueda rápida = exacta/prefix

```go
fuzzyScore() {
  if query == pkg { return 10000 }  // Exacta
  if strings.HasPrefix(pkg, query) { return 9000 }  // Prefix
  // ... vuelve a Levenshtein distance O(nm)
}
```

**Mejora potencial:**
- Índice de prefijos (trie)
- Hash map de exactas
- Binary search ordenado

---

## ⚡ OPTIMIZACIONES PROPUESTAS

### **PRIORIDAD 1: ÍNDICE LOCAL PRECALCULADO** ⭐⭐⭐

**Propuesta:** Guardar índice PyPI en caché local (archivo JSON comprimido)

```go
// internal/requirements/cache/cache.go
type IndexCache struct {
  LastUpdated time.Time
  Packages    []string  // Solo nombres, 450K strings
  CompactIndex map[string]int // pkg -> index para O(1) lookup
}

// Estrategia:
// - Primera ejecución: descarga index (15-30s, una sola vez)
// - Sesiones posteriores: cargar desde cache (100ms)
// - Cada 7 días: refresh automático (configurable)
// - ~2-5 MB comprimido en disco
```

**Beneficios:**
- ✅ Primera búsqueda: 15-30s → 100ms
- ✅ Búsqueda modal: 20s timeout → 1s timeout
- ✅ Experiencia de usuario: fluida

**Código ejemplo:**

```go
func (c *IndexCache) LoadOrFetch(ctx context.Context) ([]string, error) {
  // 1. Chequea fichero caché local
  if c.IsValid() {  // Menos de 7 días
    return c.Load()  // 100ms desde disco
  }
  
  // 2. Si no existe, descarga de PyPI
  return c.FetchAndSave()  // 15-30s, una sola vez
}

func (c *IndexCache) IsValid() bool {
  info, err := os.Stat(c.CachePath)
  age := time.Since(info.ModTime())
  return age < 7*24*time.Hour
}
```

---

### **PRIORIDAD 2: BÚSQUEDA DUAL (Local + PyPI)** ⭐⭐⭐

**Propuesta:** Búsqueda rápida local + refinamiento con metadata

```go
// FASE 1: Búsqueda RÁPIDA (local)
type SearchResult struct {
  // Solo nombres + scores
  Name  string
  Score int
}

func SearchQuick(idx []string, query string) []SearchResult {
  // Fuzzy scoring solo en 450K (1-2s)
  // Retorna TOP 25
}

// FASE 2: METADATA (parallel)
func EnrichResults(ctx context.Context, names []string) []FullResult {
  // Fetch 25x en PARALELO con goroutines + channels
  // Timeout más corto: 5s total (no 20s)
}

// USO:
results, _ := SearchQuick(index, "requests")  // 1s
full := EnrichResults(ctx, results)           // 2-4s total
```

**Beneficios:**
- ✅ Usuario ve resultados rápido (1s)
- ✅ Metadata carga en background
- ✅ Mejor UX: feedback inmediato

---

### **PRIORIDAD 3: FUZZY SCORING OPTIMIZADO** ⭐⭐

**Propuesta:** Índice de prefijos (Trie) + early termination

```go
// Estructura para búsqueda rápida de prefijos
type TrieNode struct {
  Children map[rune]*TrieNode
  Packages []string  // Nombres con este prefijo
}

func (t *Trie) SearchWithPrefix(prefix string) []string {
  // Retorna TODOS los nombres que empiezan con prefix
  // O(len(prefix)) en lugar de O(n)
  node := t.Root
  for _, ch := range prefix {
    node = node.Children[ch]
    if node == nil { return nil }
  }
  return node.Packages  // Todos los matches
}

// Uso en búsqueda:
func SearchOptimized(trie *Trie, query string) []string {
  // 1. Intenta búsqueda exacta (O(1))
  if idx, ok := exactMap[query]; ok {
    return []string{query}
  }
  
  // 2. Intenta prefix (O(len(query)))
  candidates := trie.SearchWithPrefix(query)
  
  // 3. Si pocos resultados, aplicar fuzzy
  if len(candidates) < 1000 {
    scored := fuzzyScoreList(candidates, query)
    return topN(scored, 25)
  }
  
  // 4. Si muchos, devolver TOP 25 por prefix length
  return candidates[:25]
}
```

**Beneficios:**
- ✅ Búsqueda exacta: O(1) en lugar de O(n)
- ✅ Prefix: O(len(query)) en lugar de O(n)
- ✅ Menos fuzzy scoring (solo necesario si muchos matches)

---

### **PRIORIDAD 4: BÚSQUEDA PARALELA (Metadata)** ⭐⭐

**Propuesta:** Reemplazar 25 HTTP secuenciales por paralelos

```go
// ACTUAL (secuencial, lento)
func (s *searcher) fetchPackageMetadata(names []string) []Result {
  results := make([]Result, len(names))
  for i, name := range names {
    resp, _ := http.Get(fmt.Sprintf("https://pypi.org/pypi/%s/json", name))
    results[i] = parseJSON(resp)  // Espera antes de siguiente
  }
  return results
}

// PROPUESTA (paralelo con workers)
func (s *searcher) fetchPackageMetadataParallel(ctx context.Context, names []string) []Result {
  const workers = 5  // 5 goroutines simultáneas
  
  type job struct {
    idx  int
    name string
  }
  
  jobs := make(chan job, len(names))
  results := make([]Result, len(names))
  var wg sync.WaitGroup
  
  // Workers
  for w := 0; w < workers; w++ {
    wg.Add(1)
    go func() {
      defer wg.Done()
      for job := range jobs {
        resp, _ := http.Get(...)
        results[job.idx] = parseJSON(resp)
      }
    }()
  }
  
  // Distribuir jobs
  for i, name := range names {
    jobs <- job{i, name}
  }
  
  close(jobs)
  wg.Wait()
  return results
}

// RESULTADO:
// Antes: 5s (1 req × 25) → Después: 1s (5 req simultáneos)
```

**Beneficios:**
- ✅ 25 HTTP requests: 20s → 1-2s
- ✅ Timeout puede bajar: 20s → 5s
- ✅ UX más fluido

---

### **PRIORIDAD 5: TIMEOUTS INTELIGENTES** ⭐

**Propuesta:** Reducir timeouts conservadores

```go
// ACTUAL
const (
  searchTimeout = 20 * time.Second        // ← muy mucho
  installTimeout = 60 * time.Second       // ← conservador
  listTimeout = 30 * time.Second          // ← conservador
)

// PROPUESTA
const (
  searchTimeoutQuick = 2 * time.Second     // Búsqueda local + prefix
  searchTimeoutFull = 5 * time.Second      // Con metadata paralela
  installTimeout = 20 * time.Second        // Pip install típico
  listTimeout = 5 * time.Second            // Pip list típico
)
```

**Beneficios:**
- ✅ Usuario no espera innecesariamente
- ✅ Fallback rápido a error
- ✅ Más responsivo

---

### **PRIORIDAD 6: CACHÉ DE BÚSQUEDAS RECIENTES** ⭐

**Propuesta:** Memoizar últimas 50 búsquedas

```go
type SearchCache struct {
  mu    sync.RWMutex
  cache map[string][]Result  // query → resultados
  limit int                   // máximo 50
}

func (sc *SearchCache) Search(ctx context.Context, query string) ([]Result, error) {
  // Chequea caché
  sc.mu.RLock()
  if results, ok := sc.cache[query]; ok {
    sc.mu.RUnlock()
    return results, nil  // O(1) hit
  }
  sc.mu.RUnlock()
  
  // No en caché, busca
  results := realSearch(ctx, query)
  
  // Almacena en caché
  sc.mu.Lock()
  if len(sc.cache) >= sc.limit {
    // Elimina entrada más antigua
    delete(sc.cache, oldestKey)
  }
  sc.cache[query] = results
  sc.mu.Unlock()
  
  return results, nil
}
```

**Casos de uso:**
- Usuario busca "requests", ve resultados
- Usuario borra, vuelve a escribir "requests"
- → Caché hit, resultado inmediato

**Beneficios:**
- ✅ Queries repetidas: < 1ms
- ✅ Poco overhead (50 × 4KB ≈ 200KB)

---

### **PRIORIDAD 7: PAGINACIÓN DE RESULTADOS** ⭐

**Propuesta:** Cargar TOP 25 primero, poder pedir más

```go
type PaginatedResults struct {
  Items      []Result
  Total      int
  Page       int
  PageSize   int
  HasNext    bool
}

func (s *searcher) SearchPaginated(ctx context.Context, query string, page int) (*PaginatedResults, error) {
  // Búsqueda rápida: primeros 50 (en lugar de 25)
  top50 := s.searchQuick(query)  // 1s
  
  // Retorna página actual
  start := (page - 1) * 25
  end := min(start+25, len(top50))
  
  return &PaginatedResults{
    Items:    top50[start:end],      // TOP 25 de página
    Total:    len(top50),            // Total encontrados
    Page:     page,
    PageSize: 25,
    HasNext:  end < len(top50),
  }, nil
}

// UI: mostrar "1-25 de 47 resultados"
// User presiona 'n' → página siguiente
```

**Beneficios:**
- ✅ Usuario puede ver más resultados sin esperar
- ✅ Rápido cargar primeros 50 (vs buscador como Google lento)
- ✅ UX menos frustrante

---

## 🎯 PLAN DE IMPLEMENTACIÓN

### **Fase 1: Caché Local (15min)**
```
1. Crear internal/requirements/cache/cache.go
2. Guardar índice en ~/.cache/pipnest/pypi_index.json.gz
3. Integrar en loadPackageIndex()
4. Fallback a descarga si caché viejo
```

### **Fase 2: Búsqueda Paralela (30min)**
```
1. Crear fetchPackageMetadataParallel() en search.go
2. Usar 5 workers concurrentes
3. Medir mejora: antes/después
4. Ajustar número de workers
```

### **Fase 3: Índice Trie (45min)**
```
1. Crear internal/requirements/search/trie.go
2. Construir Trie en memoria al iniciar
3. Reemplazar fuzzyScore() con búsqueda Trie+fuzzy
4. Early termination si < 100 candidatos
```

### **Fase 4: Caché de búsquedas (15min)**
```
1. Memoizar últimas 50 búsquedas
2. Hit en queries repetidas
```

### **Fase 5: Paginación (30min)**
```
1. Cambiar resultLimit a 50
2. Modal muestra página 1-25
3. Tecla 'n' → siguiente página
```

---

## 📈 RESULTADOS ESPERADOS

### **Antes:**
```
Primera búsqueda:     30s (descarga índice)
Búsquedas siguientes: 20s (fuzzy 450K)
Modal lenta:          20s timeout
25 requests metadata: 5-10s
```

### **Después:**
```
Primera búsqueda:     30s (caché) + 1s local        = 31s (una sola vez)
Búsquedas siguientes: 1s (índice caché)            = ✅
Modal rápida:         2s (búsqueda Trie)           = ✅ 10x más rápida
Metadata paralela:    1-2s (5 workers vs 1)        = ✅ 5x más rápida
Búsqueda repetida:    < 1ms (caché)                = ✅ instantáneo
```

**Tiempo total búsqueda + metadata:**
- Antes: 25s
- Después: 3-4s
- **Mejora: ~6-7x más rápida** ✅

---

## 🔧 RECOMENDACIÓN DE ORDEN

1. **Fase 1** (Caché Local) → Máximo impact, mínimo esfuerzo
2. **Fase 2** (Paralela) → Mejora dramática en metadata
3. **Fase 3** (Trie) → Búsqueda super rápida
4. **Fase 4** (Search cache) → Quick win
5. **Fase 5** (Paginación) → UX enhancement

---

## 💡 QUICK WINS INMEDIATOS

Sin cambiar código, puedes:

1. **Bajar timeouts:**
```go
searchTimeout: 20s → 5s
listTimeout:   30s → 5s  
installTimeout: 60s → 20s
```

2. **Usar búsqueda exacta primero:**
```go
func Search(query string) []Result {
  if exactMatch := findExact(allPkgs, query); exactMatch != nil {
    return []Result{exactMatch}  // O(1)
  }
  // luego fuzzy...
}
```

3. **Agregar sync.Once a loadPackageIndex:**
```go
var (
  indexOnce sync.Once
  cachedIdx []string
)

func loadPackageIndex(ctx context.Context) ([]string, error) {
  var err error
  indexOnce.Do(func() {
    cachedIdx, err = fetchPackageIndex(ctx)
  })
  return cachedIdx, err
}
```

(Probablemente ya lo hace con `sync.Once`)

---

## 📊 VALIDACIÓN

Después de cada fase:
```bash
go test -bench=BenchmarkSearch -benchtime=10s ./internal/requirements/...
go test -bench=BenchmarkFuzzyScore ./internal/requirements/...
```

Medir:
- Tiempo búsqueda (P50, P95, P99)
- Bytes en memoria (índice + caché)
- Requests HTTP (reducción)
- CPU usage

---

## ⚠️ CONSIDERACIONES

1. **Caché invalidación:** Si agrega paquetes nuevos a PyPI, no aparecen en la app hasta refresh (7 días o manual)
   - **Solución:** Botón "Refresh index" en UI

2. **Espacio en disco:** Índice comprimido ~2-5MB
   - **Solución:** Aceptable, usuario puede limpiar ~/.cache/pipnest

3. **Goroutines:** 5 workers paralelos × N búsquedas = potencial flood de conexiones
   - **Solución:** Pool limitado de connections HTTP (client.Transport.MaxIdleConns = 10)

4. **Cambio de comportamiento:** Búsqueda exacta retorna solo 1 resultado
   - **Solución:** Aceptable, usuario puede presionar ↓ para más

