FROM eu.gcr.io/kyma-project/tpi/k8s-tools:20211022-85284bf9

LABEL source=git@github.com:kyma-project/kyma.git

COPY . /app

RUN addgroup -S xipgroup && adduser -S xip --uid 2000 -G xipgroup -h /home/xip && chown -R xip /app
USER 2000

ENTRYPOINT [ "/app/xip-patch.sh" ]
