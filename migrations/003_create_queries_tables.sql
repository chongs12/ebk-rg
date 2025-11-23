-- Create queries table
CREATE TABLE IF NOT EXISTS queries (
    id CHAR(36) PRIMARY KEY,
    user_id CHAR(36) NOT NULL,
    query_text TEXT NOT NULL,
    query_type VARCHAR(50) DEFAULT 'rag',
    response LONGTEXT,
    is_answered BOOLEAN DEFAULT FALSE,
    confidence DOUBLE DEFAULT 0,
    source_count INT DEFAULT 0,
    processing_time BIGINT DEFAULT 0,
    status VARCHAR(20) DEFAULT 'pending',
    error TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_queries_user_id (user_id),
    INDEX idx_queries_status (status),
    INDEX idx_queries_query_type (query_type),
    INDEX idx_queries_created_at (created_at),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

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
    INDEX idx_sources_query_id (query_id),
    INDEX idx_sources_document_id (document_id),
    INDEX idx_sources_chunk_id (chunk_id),
    INDEX idx_sources_relevance_score (relevance_score),
    FOREIGN KEY (query_id) REFERENCES queries(id) ON DELETE CASCADE,
    FOREIGN KEY (document_id) REFERENCES documents(id) ON DELETE CASCADE,
    FOREIGN KEY (chunk_id) REFERENCES document_chunks(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Create triggers for updated_at
DELIMITER $$
CREATE TRIGGER update_queries_updated_at 
    BEFORE UPDATE ON queries 
    FOR EACH ROW 
BEGIN
    SET NEW.updated_at = CURRENT_TIMESTAMP;
END$$
DELIMITER ;