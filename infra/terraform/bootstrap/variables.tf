variable "region" {
  type    = string
  default = "us-east-1"
}

variable "github_oidc_subject" {
  description = <<-EOT
    The repository part of the GitHub OIDC `sub` claim the CI roles trust.
    This repository uses GitHub's immutable subject claims, which embed the
    owner and repository ids (owner@id/name@id), so a renamed or recreated
    repository can't inherit the roles. Read the current value with
    `gh api repos/<owner>/<repo>/actions/oidc/customization/sub`
    (sub_claim_prefix).
  EOT
  type        = string
  default     = "repo:CompositeCode@38675098/substratalapps.com@1403762290"
}

variable "billing_email" {
  description = "Where budget, anomaly, and billing-alarm alerts go"
  type        = string
}

variable "warn_budget_usd" {
  description = "Warn budget: alerts at 50/80/100%, sized just above the ~$45–55 Tier 0 floor"
  type        = number
  default     = 75
}

variable "hard_budget_usd" {
  description = "Hard budget: the circuit-breaker action fires at 100%"
  type        = number
  default     = 150
}
