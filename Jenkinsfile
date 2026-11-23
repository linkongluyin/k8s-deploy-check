pipeline {
    agent any
    options {
        ansiColor('xterm')
        timeout(
            time: 30,
            unit: 'MINUTES'
        )
        buildDiscarder(
            logRotator(
                numToKeepStr: '30'
            )
        )
    }
    environment {
        mavenHome = tool 'mvn'
        GIT_BRANCH = "${APP_NAME}_settlement"
        DOCKER_REGISTRY = "swr.cn-north-9.myhuaweicloud.com"
        DOCKER_NAMESPACE = "wlyd-delta"
        DOCKER_IMAGE = "${DOCKER_REGISTRY}/${DOCKER_NAMESPACE}/${APP_NAME}"
        DOCKER_TAG = ""
        DEPLOY_FILE = "/mnt/nfs/script/delta/${APP_NAME}.yaml"
        DOCKER_FILE = "./docker/Dockerfile"
    }
    stages {
        stage('加载配置') {
            steps {
                script {
                    echo "✅ 当前项目: ${APP_NAME}"
                    echo "📦 Git 分支: ${GIT_BRANCH}"
                    echo "📂 项目路径: ${PROJECT_PATH}"
                    echo "📄 Deployment 文件: ${DEPLOY_FILE}"
                }
            }
        }
        stage('Git Pull') {
            when { expression { return params.BUILD } }
            steps {
                script {
                    echo "\u001B[34m[拉取代码] ===========开始拉取仓库内容===========\u001B[0m"

                    dir("${PROJECT_PATH}") {
                        sh """
                            git checkout ${GIT_BRANCH}
                            git pull origin ${GIT_BRANCH}
                        """
                    }
                }
            }
        }
        stage('Maven && Package') {
            when { expression { return params.BUILD } }
            steps {
                script {
                    echo "\u001B[34m[编译打包代码] ===========开始构建镜像 ${APP_NAME}===========\u001B[0m"
                    dir("${PROJECT_PATH}") {
                        sh """
                            ${mavenHome}/bin/mvn -s ../settings.xml -U -Pcloud \
                            -Dmaven.repo.local=/data/mvn/delta/repository \
                            clean package -Dmaven.test.skip=true
                        """
                    }
                }
            }
        }

        stage('Docker Build & Push') {
            when { expression { return params.BUILD } }
            steps {
                script {
                    echo "\u001B[34m[构建Docker镜像] ===========开始构建镜像 ${APP_NAME}===========\u001B[0m"
                    dir("${PROJECT_PATH}") {
                        // DOCKER_TAG = sh(script: "date +%Y%m%d%H%M%S", returnStdout: true).trim()
                        DOCKER_TAG = "1.0-SNAPSHOT"
                        sh """
                            docker build -f ${DOCKER_FILE} -t ${DOCKER_IMAGE}:${DOCKER_TAG} .
                            docker push ${DOCKER_IMAGE}:${DOCKER_TAG}
                        """
                    }
                }
            }
        }

        stage('Deploy') {
            when { expression { return params.RESTART } }
            steps {
                script {
                echo "DEPLOY_FILE: ${DEPLOY_FILE}"
                echo "\u001B[34m[部署应用] ===========开始部署应用 ${APP_NAME}===========\u001B[0m"
                sh """
                    kubectl delete -f ${DEPLOY_FILE} || true
                    kubectl apply -f ${DEPLOY_FILE}
                """

                }
            }
        }

        stage('部署验证') {
            when { expression { return params.RESTART } }
            steps {
                script {
                    echo "\u001B[34m[部署验证] ===========开始验证 ${APP_NAME} 发布状态===========\u001B[0m"
                    // 配置全部来自 workspace 的 config.yaml（含飞书 webhook），验证结束后直接推送飞书卡片
                    sh """
                        sleep 10
                        k8s-deploy-check -namespace delta -deployment ${APP_NAME} -verbose
                    """
                }
            }
        }
    }

    // post {
    //     // always {
    //     //     echo "\u001B[34m[清理] ===========开始清理Jenkins工作区===========\u001B[0m"
    //     //     cleanWs(
    //     //         deleteDirs: true,
    //     //         disableDeferredWipeout: true
    //     //     )
    //     // }

    //     success {
    //         echo "✅ ${APP_NAME} 构建成功"
    //     }

    //     failure {
    //         echo "❌ ${APP_NAME} 构建失败"
    //     }

    // }

}
