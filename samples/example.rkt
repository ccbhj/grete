#lang racket

(defproduction "example rule"
 (defstruct poker_card [face #:string rank #:int])
 (let [
           ('x 'type '= poker_card)
           ('y 'type '= poker_card)])
 (defrule "larger rule" 
  :when [
         ('> $x 'rank > (field_of $y 'rank))
         ($x 'face '> (field_of $y 'face))]
  :when [
         ($x 'rank '> (field_of $y 'rank))
         ($x 'face '= (field_of $y 'face))]
  :when [
         ($x 'rank '= (field_of $y 'rank))
         ($x 'face '= (field_of $y 'face))]
  :then [
         (event "winner" $x)]))
  
