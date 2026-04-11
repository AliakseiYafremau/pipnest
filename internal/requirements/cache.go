package requirements

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// IndexCache gestiona caché persistent del índice PyPI
type IndexCache struct {
	mu        sync.RWMutex
	cachePath string
	packages  []packageNameEntry
	loaded    bool
	createdAt time.Time
}

const (
	cacheValidityDays = 7
	cacheFileName     = "pypi_index.json.gz"
)

// NewIndexCache crea nueva instancia
func NewIndexCache() *IndexCache {
	cacheDir := filepath.Join(os.Getenv("HOME"), ".cache", "pipnest")
	cachePath := filepath.Join(cacheDir, cacheFileName)
	return &IndexCache{
		cachePath: cachePath,
	}
}

// LoadOrFetch carga desde caché o descarga de PyPI
func (ic *IndexCache) LoadOrFetch() ([]packageNameEntry, error) {
	ic.mu.Lock()
	defer ic.mu.Unlock()

	// Si ya está en memoria y es válido, devuelve
	if ic.loaded && time.Since(ic.createdAt) < time.Duration(cacheValidityDays)*24*time.Hour {
		return ic.packages, nil
	}

	// Intenta cargar desde archivo
	if data, err := ic.loadFromDisk(); err == nil {
		ic.packages = data
		ic.loaded = true
		ic.createdAt = time.Now()
		fmt.Printf("✅ Índice cargado desde caché local (~%d paquetes)\n", len(data))
		return data, nil
	}

	// Si no existe o es viejo, descarga de PyPI
	fmt.Println("🔄 Descargando índice PyPI (primera vez, espera ~20s)...")
	packages, err := fetchPackageIndex()
	if err != nil {
		return nil, fmt.Errorf("error descargando índice: %w", err)
	}

	// Guarda en disco
	if err := ic.saveToDisk(packages); err != nil {
		fmt.Printf("⚠️  Advertencia: no se pudo guardar caché: %v\n", err)
		// No es error fatal, continúa sin caché
	}

	ic.packages = packages
	ic.loaded = true
	ic.createdAt = time.Now()
	fmt.Printf("✅ Índice descargado y cacheado (~%d paquetes)\n", len(packages))
	return packages, nil
}

// loadFromDisk carga índice comprimido desde archivo
func (ic *IndexCache) loadFromDisk() ([]packageNameEntry, error) {
	file, err := os.Open(ic.cachePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Chequea fecha de modificación
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}

	if time.Since(info.ModTime()) > time.Duration(cacheValidityDays)*24*time.Hour {
		return nil, fmt.Errorf("caché expirado (edad: %v días)", time.Since(info.ModTime()).Hours()/24)
	}

	// Descomprime
	gr, err := gzip.NewReader(file)
	if err != nil {
		return nil, err
	}
	defer gr.Close()

	data, err := io.ReadAll(gr)
	if err != nil {
		return nil, err
	}

	// Deserializa JSON
	var packages []packageNameEntry
	if err := json.Unmarshal(data, &packages); err != nil {
		return nil, err
	}

	return packages, nil
}

// saveToDisk guarda índice comprimido en archivo
func (ic *IndexCache) saveToDisk(packages []packageNameEntry) error {
	// Crea directorio si no existe
	dir := filepath.Dir(ic.cachePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Crea archivo temporal
	tmpPath := ic.cachePath + ".tmp"
	file, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Comprime y escribe
	gw := gzip.NewWriter(file)

	data, _ := json.Marshal(packages)
	if _, err := gw.Write(data); err != nil {
		gw.Close()
		return err
	}

	if err := gw.Close(); err != nil {
		return err
	}

	file.Close()

	// Renombra (atomic)
	err = os.Rename(tmpPath, ic.cachePath)
	if err == nil {
		// Log tamaño del archivo comprimido
		info, _ := os.Stat(ic.cachePath)
		sizeMB := float64(info.Size()) / 1024.0 / 1024.0
		fmt.Printf("💾 Caché guardado en %s (%.1f MB)\n", ic.cachePath, sizeMB)
	}
	return err
}

// InvalidateCache fuerza refresh del caché
func (ic *IndexCache) InvalidateCache() error {
	ic.mu.Lock()
	defer ic.mu.Unlock()

	ic.loaded = false
	ic.packages = nil

	// Elimina archivo caché
	_ = os.Remove(ic.cachePath)

	return nil
}

// GetCacheAge retorna antigüedad del caché en horas
func (ic *IndexCache) GetCacheAge() float64 {
	info, err := os.Stat(ic.cachePath)
	if err != nil {
		return -1
	}
	return time.Since(info.ModTime()).Hours()
}

// GetCacheStats retorna información del caché
func (ic *IndexCache) GetCacheStats() map[string]interface{} {
	ic.mu.RLock()
	defer ic.mu.RUnlock()

	info, err := os.Stat(ic.cachePath)
	if err != nil {
		return map[string]interface{}{
			"exists":  false,
			"loaded":  ic.loaded,
			"entries": len(ic.packages),
		}
	}

	return map[string]interface{}{
		"exists":    true,
		"loaded":    ic.loaded,
		"entries":   len(ic.packages),
		"path":      ic.cachePath,
		"ageHours":  time.Since(info.ModTime()).Hours(),
		"sizeMB":    float64(info.Size()) / 1024.0 / 1024.0,
		"validDays": cacheValidityDays,
	}
}
