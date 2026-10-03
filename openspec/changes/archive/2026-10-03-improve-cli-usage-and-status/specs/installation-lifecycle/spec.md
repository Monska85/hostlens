# Spec Delta

## ADDED Requirements

### Requirement: Clear external-resource notice on uninstall

The uninstall plan SHALL explain in plain language that HostLens leaves shared system journal entries and access permissions granted manually by the administrator. It SHALL tell the administrator to remove manual grants separately when no longer needed. This notice SHALL NOT imply that uninstall changes or owns those resources.

#### Scenario: Uninstall preview

- **WHEN** an administrator previews uninstall
- **THEN** the output explains what remains and what the administrator may need to remove manually

#### Scenario: Applied uninstall

- **WHEN** an administrator applies uninstall
- **THEN** HostLens preserves shared journals and untracked manual access grants under the existing ownership rules
