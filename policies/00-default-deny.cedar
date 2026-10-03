// ToolHive 0.51.0 requires at least one policy. This permit never matches,
// leaving Cedar's default-deny behavior in effect.
permit(principal, action, resource) when { false };
