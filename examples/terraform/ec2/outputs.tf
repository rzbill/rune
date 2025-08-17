output "instance_public_ip" {
  value = aws_instance.rune.public_ip
}

output "grpc_endpoint" {
  value = "${aws_instance.rune.public_ip}:7863"
}

output "http_endpoint" {
  value = "http://${aws_instance.rune.public_ip}:7861"
}


