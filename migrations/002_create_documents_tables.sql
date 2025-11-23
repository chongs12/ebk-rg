-- Create documents table
CREATE TABLE IF NOT EXISTS documents (
    id CHAR(36) PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    file_name VARCHAR(255) NOT NULL,
    file_path VARCHAR(500) NOT NULL,
    file_size BIGINT NOT NULL,
    file_type VARCHAR(50) NOT NULL,
    mime_type VARCHAR(100) NOT NULL,
    checksum VARCHAR(64) NOT NULL,
    content LONGTEXT,
    language VARCHAR(10) DEFAULT 'en',
    word_count INT DEFAULT 0,
    page_count INT DEFAULT 0,
    is_public BOOLEAN DEFAULT FALSE,
    is_processed BOOLEAN DEFAULT FALSE,
    status VARCHAR(20) DEFAULT 'pending',
    owner_id CHAR(36) NOT NULL,
    tags TEXT,
    keywords TEXT,
    category VARCHAR(100),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_documents_owner_id (owner_id),
    INDEX idx_documents_status (status),
    INDEX idx_documents_is_public (is_public),
    INDEX idx_documents_is_processed (is_processed),
    INDEX idx_documents_created_at (created_at),
    FOREIGN KEY (owner_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Create document_chunks table
CREATE TABLE IF NOT EXISTS document_chunks (
    id CHAR(36) PRIMARY KEY,
    document_id CHAR(36) NOT NULL,
    chunk_index INT NOT NULL,
    content TEXT NOT NULL,
    start_pos INT NOT NULL,
    end_pos INT NOT NULL,
    word_count INT DEFAULT 0,
    char_count INT DEFAULT 0,
    vector TEXT,
    keywords TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_chunks_document_id (document_id),
    INDEX idx_chunks_chunk_index (chunk_index),
    FOREIGN KEY (document_id) REFERENCES documents(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Create document_permissions table
CREATE TABLE IF NOT EXISTS document_permissions (
    id CHAR(36) PRIMARY KEY,
    document_id CHAR(36) NOT NULL,
    user_id CHAR(36) NOT NULL,
    permission VARCHAR(20) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY unique_doc_user_permission (document_id, user_id, permission),
    INDEX idx_permissions_document_id (document_id),
    INDEX idx_permissions_user_id (user_id),
    FOREIGN KEY (document_id) REFERENCES documents(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Create triggers for updated_at
DELIMITER $$
CREATE TRIGGER update_documents_updated_at 
    BEFORE UPDATE ON documents 
    FOR EACH ROW 
BEGIN
    SET NEW.updated_at = CURRENT_TIMESTAMP;
END$$

CREATE TRIGGER update_document_chunks_updated_at 
    BEFORE UPDATE ON document_chunks 
    FOR EACH ROW 
BEGIN
    SET NEW.updated_at = CURRENT_TIMESTAMP;
END$$

CREATE TRIGGER update_document_permissions_updated_at 
    BEFORE UPDATE ON document_permissions 
    FOR EACH ROW 
BEGIN
    SET NEW.updated_at = CURRENT_TIMESTAMP;
END$$
DELIMITER ;