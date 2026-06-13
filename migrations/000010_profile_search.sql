CREATE VIRTUAL TABLE profile_search USING fts5(
    profile_id UNINDEXED,
    entity_type UNINDEXED,
    entity_id UNINDEXED,
    document_id UNINDEXED,
    source_chunk_id UNINDEXED,
    document_name UNINDEXED,
    title,
    body,
    tokenize = 'trigram'
);

INSERT INTO profile_search(
    profile_id,
    entity_type,
    entity_id,
    document_id,
    source_chunk_id,
    document_name,
    title,
    body
)
SELECT
    sd.profile_id,
    'source_chunk',
    sc.id,
    sd.id,
    sc.id,
    sd.original_name,
    sd.original_name,
    sc.text
FROM source_chunks sc
JOIN source_documents sd ON sd.id = sc.document_id;

INSERT INTO profile_search(
    profile_id,
    entity_type,
    entity_id,
    document_id,
    source_chunk_id,
    document_name,
    title,
    body
)
SELECT
    e.profile_id,
    'evidence',
    e.id,
    sd.id,
    sc.id,
    sd.original_name,
    e.title,
    e.content
FROM evidence e
JOIN evidence_sources es ON es.evidence_id = e.id
JOIN source_chunks sc ON sc.id = es.chunk_id
JOIN source_documents sd ON sd.id = sc.document_id;

CREATE TRIGGER profile_search_source_chunk_insert
AFTER INSERT ON source_chunks
BEGIN
    INSERT INTO profile_search(
        profile_id,
        entity_type,
        entity_id,
        document_id,
        source_chunk_id,
        document_name,
        title,
        body
    )
    SELECT
        sd.profile_id,
        'source_chunk',
        NEW.id,
        sd.id,
        NEW.id,
        sd.original_name,
        sd.original_name,
        NEW.text
    FROM source_documents sd
    WHERE sd.id = NEW.document_id;
END;

CREATE TRIGGER profile_search_source_chunk_update
AFTER UPDATE ON source_chunks
BEGIN
    DELETE FROM profile_search
    WHERE source_chunk_id = OLD.id;

    INSERT INTO profile_search(
        profile_id,
        entity_type,
        entity_id,
        document_id,
        source_chunk_id,
        document_name,
        title,
        body
    )
    SELECT
        sd.profile_id,
        'source_chunk',
        NEW.id,
        sd.id,
        NEW.id,
        sd.original_name,
        sd.original_name,
        NEW.text
    FROM source_documents sd
    WHERE sd.id = NEW.document_id;

    INSERT INTO profile_search(
        profile_id,
        entity_type,
        entity_id,
        document_id,
        source_chunk_id,
        document_name,
        title,
        body
    )
    SELECT
        e.profile_id,
        'evidence',
        e.id,
        sd.id,
        NEW.id,
        sd.original_name,
        e.title,
        e.content
    FROM evidence_sources es
    JOIN evidence e ON e.id = es.evidence_id
    JOIN source_documents sd ON sd.id = NEW.document_id
    WHERE es.chunk_id = NEW.id;
END;

CREATE TRIGGER profile_search_source_chunk_delete
AFTER DELETE ON source_chunks
BEGIN
    DELETE FROM profile_search
    WHERE source_chunk_id = OLD.id;
END;

CREATE TRIGGER profile_search_evidence_source_insert
AFTER INSERT ON evidence_sources
BEGIN
    INSERT INTO profile_search(
        profile_id,
        entity_type,
        entity_id,
        document_id,
        source_chunk_id,
        document_name,
        title,
        body
    )
    SELECT
        e.profile_id,
        'evidence',
        e.id,
        sd.id,
        sc.id,
        sd.original_name,
        e.title,
        e.content
    FROM evidence e
    JOIN source_chunks sc ON sc.id = NEW.chunk_id
    JOIN source_documents sd ON sd.id = sc.document_id
    WHERE e.id = NEW.evidence_id;
END;

CREATE TRIGGER profile_search_evidence_source_delete
AFTER DELETE ON evidence_sources
BEGIN
    DELETE FROM profile_search
    WHERE entity_type = 'evidence'
      AND entity_id = OLD.evidence_id
      AND source_chunk_id = OLD.chunk_id;
END;

CREATE TRIGGER profile_search_evidence_update
AFTER UPDATE ON evidence
BEGIN
    UPDATE profile_search
    SET profile_id = NEW.profile_id,
        title = NEW.title,
        body = NEW.content
    WHERE entity_type = 'evidence'
      AND entity_id = NEW.id;
END;

CREATE TRIGGER profile_search_evidence_delete
AFTER DELETE ON evidence
BEGIN
    DELETE FROM profile_search
    WHERE entity_type = 'evidence'
      AND entity_id = OLD.id;
END;

CREATE TRIGGER profile_search_document_update
AFTER UPDATE OF profile_id, original_name ON source_documents
BEGIN
    UPDATE profile_search
    SET profile_id = NEW.profile_id,
        document_name = NEW.original_name,
        title = CASE
            WHEN entity_type = 'source_chunk' THEN NEW.original_name
            ELSE title
        END
    WHERE document_id = NEW.id;
END;

CREATE TRIGGER profile_search_document_delete
AFTER DELETE ON source_documents
BEGIN
    DELETE FROM profile_search
    WHERE document_id = OLD.id;
END;
