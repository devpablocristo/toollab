# GCP Connection Guide

Este archivo es solo una guía de conexión a GCP para usar desde CLI.
No contiene credenciales, datos de base de datos ni información de proyectos específicos.

## 1) Login en GCP
```bash
gcloud auth login
```

## 2) Ver cuentas autenticadas
```bash
gcloud auth list
```

## 3) Seleccionar cuenta activa (opcional)
```bash
gcloud config set account TU_CUENTA@MAIL.COM
```

## 4) Configurar proyecto activo
```bash
gcloud config set project TU_PROJECT_ID
```

## 5) Verificar contexto actual
```bash
gcloud config list
```
