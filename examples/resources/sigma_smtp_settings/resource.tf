resource "sigma_smtp_settings" "main" {
  host         = "smtp.example.com"
  port         = "587"
  username     = "sigma-mailer"
  password     = var.smtp_password
  sender_email = "no-reply@example.com"
  sender_name  = "Sigma"
}
