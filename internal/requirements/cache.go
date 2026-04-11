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
	home := os.Getenv("HOME")
	if home == "" {
		// Fallback si HOME no está definido
		home = os.Getenv("USERPROFILE")
		if home == "" {
			// Último recurso: usa directorio actual
			home = "."
		}
	}

	cacheDir := filepath.Join(home, ".cache", "pipnest")
	cachePath := filepath.Join(cacheDir, cacheFileName)

	// Intenta crear el directorio de caché desde el inicio
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		fmt.Printf("⚠️  No se pudo crear directorio caché: %v\n", err)
	}

	return &IndexCache{
		cachePath: cachePath,
	}
}

// LoadOrFetch carga desde caché o descarga de PyPI
func (ic *IndexCache) LoadOrFetch() ([]packageNameEntry, error) {
	ic.mu.Lock()
	defer ic.mu.Unlock()

	fmt.Printf("🔍 Intentando cargar caché: %s\n", ic.cachePath)

	// Si ya está en memoria y es válido, devuelve
	if ic.loaded && time.Since(ic.createdAt) < time.Duration(cacheValidityDays)*24*time.Hour {
		fmt.Printf("✅ Caché en memoria válido (~%d paquetes)\n", len(ic.packages))
		return ic.packages, nil
	}

	// Intenta cargar desde archivo
	if data, err := ic.loadFromDisk(); err == nil {
		ic.packages = data
		ic.loaded = true
		ic.createdAt = time.Now()
		fmt.Printf("✅ Índice cargado desde caché local (~%d paquetes)\n", len(data))
		return data, nil
	} else {
		fmt.Printf("ℹ️  No hay caché válido en disco: %v\n", err)
	}

	// Si no existe o es viejo, descarga de PyPI
	fmt.Println("🔄 Descargando índice PyPI (primera vez, espera ~20-30s)...")
	packages, err := fetchPackageIndex()
	if err != nil {
		fmt.Printf("❌ Error descargando índice: %v\n", err)
		return nil, fmt.Errorf("error descargando índice: %w", err)
	}

	fmt.Printf("✅ Conexión exitosa a PyPI (%d paquetes descargados)\n", len(packages))

	// Guarda en disco
	fmt.Printf("💾 Intentando guardar caché en: %s\n", ic.cachePath)
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
	fmt.Printf("📁 Verificando directorio: %s\n", dir)

	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Printf("❌ Error creando directorio: %v\n", err)
		return err
	}
	fmt.Printf("✅ Directorio listo: %s\n", dir)

	// Crea archivo temporal
	tmpPath := ic.cachePath + ".tmp"
	fmt.Printf("📝 Creando archivo temporal: %s\n", tmpPath)

	file, err := os.Create(tmpPath)
	if err != nil {
		fmt.Printf("❌ Error creando archivo temporal: %v\n", err)
		return err
	}
	defer file.Close()

	// Comprime y escribe
	fmt.Println("🗜️  Comprimiendo datos...")
	gw := gzip.NewWriter(file)

	data, _ := json.Marshal(packages)
	fmt.Printf("📦 Datos sin comprimir: ~%d bytes\n", len(data))

	if _, err := gw.Write(data); err != nil {
		fmt.Printf("❌ Error escribiendo datos: %v\n", err)
		gw.Close()
		return err
	}

	if err := gw.Close(); err != nil {
		fmt.Printf("❌ Error cerrando compresor: %v\n", err)
		return err
	}

	file.Close()

	// Renombra (atomic)
	fmt.Printf("🔄 Moviendo archivo: %s → %s\n", tmpPath, ic.cachePath)
	err = os.Rename(tmpPath, ic.cachePath)
	if err != nil {
		fmt.Printf("❌ Error renombrando archivo: %v\n", err)
		return err
	}

	// Log tamaño del archivo comprimido
	info, err := os.Stat(ic.cachePath)
	if err == nil {
		sizeMB := float64(info.Size()) / 1024.0 / 1024.0
		fmt.Printf("✅ Caché guardado exitosamente: %s (%.1f MB)\n", ic.cachePath, sizeMB)
	}

	return nil
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
