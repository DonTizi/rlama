package service

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

// EmbeddingCache provides caching for embeddings to avoid regeneration of the same content
type EmbeddingCache struct {
	cache       map[string]CachedEmbedding
	mutex       sync.RWMutex
	maxSize     int           // Taille maximale du cache
	ttl         time.Duration // Durée de vie des entrées
	lastCleanup time.Time     // Dernière fois que le cache a été nettoyé
}

// CachedEmbedding represents a cached embedding with metadata
type CachedEmbedding struct {
	Embedding  []float32
	CreatedAt  time.Time
	AccessedAt time.Time
	UseCount   int // Pour garder trace des éléments les plus utilisés
}

// NewEmbeddingCache creates a new embedding cache
func NewEmbeddingCache(maxSize int, ttl time.Duration) *EmbeddingCache {
	return &EmbeddingCache{
		cache:       make(map[string]CachedEmbedding),
		maxSize:     maxSize,
		ttl:         ttl,
		lastCleanup: time.Now(),
	}
}

// Get retrieves an embedding from the cache
func (c *EmbeddingCache) Get(text string, modelName string) ([]float32, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	key := c.generateKey(text, modelName)
	cached, exists := c.cache[key]

	if !exists {
		return nil, false
	}

	// Vérifier si l'entrée est expirée
	if time.Since(cached.CreatedAt) > c.ttl {
		return nil, false
	}

	// Mettre à jour les statistiques d'accès (sans lock d'écriture pour la performance)
	cached.AccessedAt = time.Now()
	cached.UseCount++
	c.cache[key] = cached

	return cached.Embedding, true
}

// Set adds an embedding to the cache
func (c *EmbeddingCache) Set(text string, modelName string, embedding []float32) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Vérifier si le cache doit être nettoyé
	if time.Since(c.lastCleanup) > c.ttl/2 {
		c.cleanup()
		c.lastCleanup = time.Now()
	}

	// Ajouter au cache
	key := c.generateKey(text, modelName)
	c.cache[key] = CachedEmbedding{
		Embedding:  embedding,
		CreatedAt:  time.Now(),
		AccessedAt: time.Now(),
		UseCount:   1,
	}
}

// generateKey creates a unique key for the cache
func (c *EmbeddingCache) generateKey(text string, modelName string) string {
	hasher := sha256.New()
	hasher.Write([]byte(text))
	hasher.Write([]byte(modelName))
	return hex.EncodeToString(hasher.Sum(nil))
}

// cleanup removes expired or least used entries when cache is full
func (c *EmbeddingCache) cleanup() {
	now := time.Now()

	// Supprimer les entrées expirées
	for key, entry := range c.cache {
		if now.Sub(entry.CreatedAt) > c.ttl {
			delete(c.cache, key)
		}
	}

	// Si le cache est toujours trop grand, supprimer les entrées les moins utilisées
	if len(c.cache) > c.maxSize {
		type keyScore struct {
			key   string
			score float64 // Combined score (usage and recency)
		}

		scores := make([]keyScore, 0, len(c.cache))

		// Calculate a score for each entry (combined usage and recency)
		for key, entry := range c.cache {
			recencyScore := float64(now.Sub(entry.AccessedAt)) / float64(c.ttl)
			usageScore := 1.0 / float64(1+entry.UseCount) // Higher usage = smaller score
			combinedScore := recencyScore * usageScore    // Smaller = better

			scores = append(scores, keyScore{key, combinedScore})
		}

		// Sort by score
		for i := 0; i < len(scores); i++ {
			for j := i + 1; j < len(scores); j++ {
				if scores[i].score < scores[j].score {
					scores[i], scores[j] = scores[j], scores[i]
				}
			}
		}

		// Remove entries with the highest score (least useful)
		toRemove := len(c.cache) - c.maxSize
		for i := 0; i < toRemove; i++ {
			delete(c.cache, scores[i].key)
		}
	}
}
