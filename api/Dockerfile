FROM mcr.microsoft.com/dotnet/sdk:10.0-alpine AS build
WORKDIR /src

COPY Directory.Build.props .editorconfig ./
COPY town-crier.sln ./
COPY src/town-crier.domain/town-crier.domain.csproj src/town-crier.domain/
COPY src/town-crier.application/town-crier.application.csproj src/town-crier.application/
COPY src/town-crier.infrastructure/town-crier.infrastructure.csproj src/town-crier.infrastructure/
COPY src/town-crier.web/town-crier.web.csproj src/town-crier.web/

RUN dotnet restore src/town-crier.web/town-crier.web.csproj

COPY src/ src/
RUN dotnet publish src/town-crier.web/town-crier.web.csproj \
    -c Release \
    -o /app \
    --no-restore

FROM mcr.microsoft.com/dotnet/runtime-deps:10.0-alpine AS runtime
WORKDIR /app

RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser

COPY --from=build /app .

ENV ASPNETCORE_URLS=http://+:8080
EXPOSE 8080

ENTRYPOINT ["./town-crier.web"]
