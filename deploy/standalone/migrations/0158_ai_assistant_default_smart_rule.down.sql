-- The system-key column and immutable smart-analysis history are consumed by
-- later migrations and persisted insight runs. Refuse rollback rather than
-- deleting traceability or reviving independently writable rule resources.
SIGNAL SQLSTATE '45000'
  SET MESSAGE_TEXT = '0158 rollback blocked: preserve immutable smart-analysis history';
