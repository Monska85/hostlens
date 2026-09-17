# ui-ready-typed-tool-contract

Make HostLens consumable by a schema-driven desktop client: derive every MCP tool's input and output JSON Schema from one typed Go definition, advertise `outputSchema` and specific descriptions in discovery, validate results against that schema before returning them, publish a machine-readable contract snapshot, and delete the hand-written schema tables, the shared argument union, and the untyped result maps they replace.
