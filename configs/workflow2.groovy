pipeline {
    agent none
    stages {
        stage('Example Build') {
            agent { docker 'maven:3.9.3-eclipse-temurin-17' }
            steps {
                echo 'Hello, Maven'
                sh 'mvn --version'
                git branch: 'master', 
                    credentialsId: '12345-1234-4696-af25-123455',
                    url: 'ssh://git@bitbucket.org:company/repo.git'
            }
        }
        stage('Example Test') {
            agent { docker 'openjdk:17-jdk' }
            steps {
                echo 'Hello, JDK'
                sh 'java -version'
            }
        }
        stage('Parallel Stage') {
            failFast true
            parallel {
                stage('Branch A') {
                    steps {
                        echo 'On Branch A'
                    }
                }
                stage('Branch B') {
                    steps {
                        echo 'On Branch B'
                    }
                }
                stage('Branch C') {
                    steps {
                        echo 'In stage Nested 1 within Branch C'
                    }
                }
            }
        }
    }
}