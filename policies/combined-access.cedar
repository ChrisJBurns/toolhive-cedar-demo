permit(principal in THVGroup::"engineering", action == Action::"call_tool", resource == Tool::"list_resources");
permit(principal in THVGroup::"support", action == Action::"call_tool", resource == Tool::"add_issue_comment");
permit(principal in THVGroup::"support", action == Action::"call_tool", resource == Tool::"issue_read");
