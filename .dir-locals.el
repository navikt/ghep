;;; Directory Local Variables
;;; For more information see (info "(emacs) Directory Variables")

((go-mode
  . ((dape-configs
      . ((ghep-debug
          modes (go-mode go-ts-mode)
          command "mise"
          command-args ("run" "debug")
          host "127.0.0.1"
          port 55878
          :type "go"
          :request "launch"
          :mode "debug"
          :program "."
          :stopOnEntry nil)))))
 (go-ts-mode
  . ((dape-configs
      . ((ghep-debug
          modes (go-mode go-ts-mode)
          command "mise"
          command-args ("run" "debug")
          host "127.0.0.1"
          port 55878
          :type "go"
          :request "launch"
          :mode "debug"
          :program "."
          :stopOnEntry nil)))))))
