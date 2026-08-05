-- Deliberately non-destructive. The reconciliation may create records that
-- have acquired business references; rollback must never delete tenant data.
SELECT 1;
