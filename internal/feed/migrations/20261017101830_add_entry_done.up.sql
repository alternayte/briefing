-- NOTE: ALTER TYPE ... ADD VALUE cannot run inside a transaction; run this migration outside a transaction block
ALTER TYPE "entrystate" ADD VALUE 'done';