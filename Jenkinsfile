pipeline {
    agent { docker { image 'golang:1.22.5-alpine3.20' } }
    options {
        skipStagesAfterUnstable()
    }
    stages {
        stage('Build') {
            steps {
                sh 'go version'
                sh 'go build -v -o epcache'
            }
        }
        stage('Test') {
            steps {
                sh 'go test -v ./...'
            }
        }
        stage('Archive') {
            steps {
                sh 'tar -czf epcache.tar.gz epcache'
                archiveArtifacts 'epcache.tar.gz'
            }
        }
    }
    post {
        always {
            cleanWs()
        }
        success {
            echo 'Succeeded!'
        }
        fail {
            echo 'Failed'
        }
    }
}
