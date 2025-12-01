-- Create query_sources table
CREATE TABLE IF NOT EXISTS query_sources (
    id CHAR(36) PRIMARY KEY,
    query_id CHAR(36) NOT NULL,
    document_id CHAR(36) NOT NULL,
    chunk_id CHAR(36) NOT NULL,
    relevance_score DOUBLE DEFAULT 0,
    excerpt TEXT,
    position INT DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    -- Indexes (aligned with your queries table style)
    INDEX idx_sources_query_id (query_id),
    INDEX idx_sources_document_id (document_id),
    INDEX idx_sources_chunk_id (chunk_id),
    INDEX idx_sources_relevance_score (relevance_score),
    
    -- Foreign keys (assuming documents and document_chunks tables exist)
    FOREIGN KEY (query_id) REFERENCES queries(id) ON DELETE CASCADE,
    FOREIGN KEY (document_id) REFERENCES documents(id) ON DELETE CASCADE,
    FOREIGN KEY (chunk_id) REFERENCES text_chunks(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;