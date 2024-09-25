workspace "E-Quiz" "Description" {

    !identifiers hierarchical

    model {
        participant = person "Participant"
        equiz = softwareSystem "E-Quiz System" {
            fe = container "Frontend" {
                tags = "Mobile App"
            }
            api = container "API Gateway" "Routing, Security, Monitor...\nWebsocket for sending updated data in real-time" "Go"
            core = container "Core" "Business Logic" "Go" {
                api = component "Core API" "Serve API, act as a coordinator to handle incoming requests."
                quizSession = component "Quiz Session" "Manage quiz's sessions.\nAPIs: PrepareQuiz, JoinSession, GetCurrentQuestion\nEvents: QuizStarted, QuizEnded, QuestionStarted, QuestionEnded"
                score = component "Score" "Manage user scores.\nAPIs: IncreaseScore\nEvents: ScoreUpdated"
                leaderboard = component "Leaderboard" "Manage leaderboard.\nAPIs: GetLeaderboard\nEvents: LeaderboardUpdated"
                eb = component "Event Bus" "An in-memory event bus" {
                    tags = "PubSub"
                }
            }
            quizSessionDB = container "Quiz Session Database" "Postgres" {
                tags = "Database"
            }
            scoreDB = container "Score Database" "Postgres" {
                tags = "Database"
            }
            leaderboardDB = container "Leaderboard Database" "Redis" {
                tags = "Database"
            }
            pubsub = container "PubSub" "Redis" {
                tags = "PubSub"
            }
        }

        participant -> equiz.fe "Use"
        equiz.fe -> equiz.api "Use API / Websocket" "HTTPS"
        equiz.pubsub -> equiz.api "Dispatch Events"
        equiz.api -> equiz.core.api "Use API" "gRPC"
        equiz.core.api -> equiz.core.quizSession "Use"
        equiz.core.api -> equiz.core.score "Use"
        equiz.core.api -> equiz.core.leaderboard "Use"
        equiz.core.api -> equiz.pubsub "Publish Events"
        equiz.core.quizSession -> equiz.core.eb "Publish Events" {
            tags = "Direct"
        }
        equiz.core.score -> equiz.core.eb "Publish Events"
        equiz.core.leaderboard -> equiz.core.eb "Publish Events"
        equiz.core.eb -> equiz.core.leaderboard "Dispatch Events\nScoreUpdated, QuizEnded, QuestionEnded" {
            tags = "Direct"
        }
        equiz.core.eb -> equiz.core.api "Dispatch Events"
        equiz.core.quizSession -> equiz.quizSessionDB "Read/Write"
        equiz.core.score -> equiz.scoreDB "Read/Write"
        equiz.core.leaderboard -> equiz.leaderboardDB "Read/Write"
    }

    views {
        systemContext equiz "SystemContext" {
            include *
            autolayout lr
        }

        container equiz "Container" {
            include *
            exclude equiz.quizSessionDB
            exclude equiz.scoreDB
            exclude equiz.leaderboardDB
        }

        component equiz.core "Component" {
            include *
        }

        styles {
            element "Element" {
                color #ffffff
            }
            element "Person" {
                background #4379F2
                shape person
            }
            element "Software System" {
                background #4379F2
            }
            element "Mobile App" {
                shape MobileDevicePortrait
                background #4379F2
            }
            element "Container" {
                background #4379F2
            }
            element "Component" {
                background #4379F2
            }
            element "Database" {
                shape cylinder
                height 200
                width 250
            }
            element "PubSub" {
                shape pipe
                height 150
            }
            relationship "Relationship" {
                routing Orthogonal
            }
            relationship "Direct" {
                routing Direct
            }
        }
    }

    configuration {
        scope softwaresystem
    }

}